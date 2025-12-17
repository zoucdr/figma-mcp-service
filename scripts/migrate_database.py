#!/usr/bin/env python3
"""
Figma Deliver 数据库迁移脚本

将数据从源数据库迁移到目标数据库
"""

import sys
import pymysql
from pymysql.cursors import DictCursor
from datetime import datetime
import argparse


class DatabaseMigrator:
    """数据库迁移工具"""
    
    def __init__(self, source_config, target_config):
        """
        初始化迁移工具
        
        Args:
            source_config: 源数据库配置字典
            target_config: 目标数据库配置字典
        """
        self.source_config = source_config
        self.target_config = target_config
        self.source_conn = None
        self.target_conn = None
        
    def connect(self):
        """连接到源数据库和目标数据库"""
        try:
            print(f"[{self._timestamp()}] 正在连接源数据库...")
            self.source_conn = pymysql.connect(
                host=self.source_config['host'],
                port=self.source_config['port'],
                user=self.source_config['user'],
                password=self.source_config['password'],
                database=self.source_config['database'],
                charset='utf8mb4',
                cursorclass=DictCursor
            )
            print(f"[{self._timestamp()}] ✓ 源数据库连接成功")
            
            print(f"[{self._timestamp()}] 正在连接目标数据库...")
            self.target_conn = pymysql.connect(
                host=self.target_config['host'],
                port=self.target_config['port'],
                user=self.target_config['user'],
                password=self.target_config['password'],
                database=self.target_config['database'],
                charset='utf8mb4',
                cursorclass=DictCursor
            )
            print(f"[{self._timestamp()}] ✓ 目标数据库连接成功")
            
        except Exception as e:
            print(f"[{self._timestamp()}] ✗ 数据库连接失败: {e}")
            sys.exit(1)
    
    def close(self):
        """关闭数据库连接"""
        if self.source_conn:
            self.source_conn.close()
            print(f"[{self._timestamp()}] 源数据库连接已关闭")
        if self.target_conn:
            self.target_conn.close()
            print(f"[{self._timestamp()}] 目标数据库连接已关闭")
    
    def get_tables(self):
        """获取源数据库中的所有表"""
        cursor = self.source_conn.cursor()
        cursor.execute("SHOW TABLES")
        tables = [list(table.values())[0] for table in cursor.fetchall()]
        cursor.close()
        return tables
    
    def sort_tables_by_dependencies(self, tables):
        """根据外键依赖关系对表进行排序"""
        # 定义表的依赖顺序（父表在前，子表在后）
        # 基于 Figma Deliver 的数据库结构
        dependency_order = {
            'users': 0,                    # 最基础的表，没有依赖
            'figma_projects': 1,           # 依赖 users
            'figma_nodes': 2,              # 依赖 figma_projects
            'mcp_call_logs': 2,            # 依赖 users
            'mcp_connections': 1,          # 依赖 users
            'prompt_shares': 1,            # 依赖 users
            'prompt_likes': 2,             # 依赖 users 和 prompt_shares
        }
        
        # 对表进行排序
        sorted_tables = sorted(tables, key=lambda t: dependency_order.get(t, 999))
        return sorted_tables
    
    def get_table_create_sql(self, table_name):
        """获取表的创建SQL语句"""
        cursor = self.source_conn.cursor()
        cursor.execute(f"SHOW CREATE TABLE `{table_name}`")
        result = cursor.fetchone()
        cursor.close()
        return list(result.values())[1]
    
    def get_table_data(self, table_name):
        """获取表的所有数据"""
        cursor = self.source_conn.cursor()
        cursor.execute(f"SELECT * FROM `{table_name}`")
        data = cursor.fetchall()
        cursor.close()
        return data
    
    def disable_foreign_key_checks(self):
        """禁用外键检查"""
        cursor = self.target_conn.cursor()
        try:
            cursor.execute("SET FOREIGN_KEY_CHECKS=0")
            self.target_conn.commit()
            print(f"[{self._timestamp()}] 已禁用外键检查")
        except Exception as e:
            print(f"[{self._timestamp()}] 禁用外键检查失败: {e}")
        finally:
            cursor.close()
    
    def enable_foreign_key_checks(self):
        """启用外键检查"""
        cursor = self.target_conn.cursor()
        try:
            cursor.execute("SET FOREIGN_KEY_CHECKS=1")
            self.target_conn.commit()
            print(f"[{self._timestamp()}] 已启用外键检查")
        except Exception as e:
            print(f"[{self._timestamp()}] 启用外键检查失败: {e}")
        finally:
            cursor.close()
    
    def create_table(self, table_name, create_sql):
        """在目标数据库中创建表"""
        cursor = self.target_conn.cursor()
        try:
            # 先删除表（如果存在）
            cursor.execute(f"DROP TABLE IF EXISTS `{table_name}`")
            
            # 处理索引键长度问题 - 将 VARCHAR(255) 的索引改为 VARCHAR(191)
            if 'mcp_connections' in table_name.lower():
                create_sql = create_sql.replace('VARCHAR(255)', 'VARCHAR(191)')
            
            # 创建表
            cursor.execute(create_sql)
            self.target_conn.commit()
            print(f"[{self._timestamp()}] ✓ 表 {table_name} 创建成功")
            return True
        except Exception as e:
            print(f"[{self._timestamp()}] ✗ 表 {table_name} 创建失败: {e}")
            self.target_conn.rollback()
            return False
        finally:
            cursor.close()
    
    def insert_data(self, table_name, data):
        """向目标数据库表中插入数据"""
        if not data:
            print(f"[{self._timestamp()}] ⊙ 表 {table_name} 无数据，跳过")
            return True
        
        cursor = self.target_conn.cursor()
        try:
            # 获取列名
            columns = list(data[0].keys())
            placeholders = ', '.join(['%s'] * len(columns))
            columns_str = ', '.join([f"`{col}`" for col in columns])
            
            # 构建INSERT语句
            sql = f"INSERT INTO `{table_name}` ({columns_str}) VALUES ({placeholders})"
            
            # 批量插入数据
            values = [tuple(row.values()) for row in data]
            cursor.executemany(sql, values)
            self.target_conn.commit()
            
            print(f"[{self._timestamp()}] ✓ 表 {table_name} 数据插入成功 ({len(data)} 条)")
            return True
        except Exception as e:
            print(f"[{self._timestamp()}] ✗ 表 {table_name} 数据插入失败: {e}")
            self.target_conn.rollback()
            return False
        finally:
            cursor.close()
    
    def migrate_table(self, table_name):
        """迁移单个表"""
        print(f"\n[{self._timestamp()}] 开始迁移表: {table_name}")
        print(f"[{self._timestamp()}] " + "=" * 60)
        
        # 获取表结构
        create_sql = self.get_table_create_sql(table_name)
        
        # 在目标数据库创建表
        if not self.create_table(table_name, create_sql):
            return False
        
        # 获取表数据
        data = self.get_table_data(table_name)
        
        # 插入数据
        if not self.insert_data(table_name, data):
            return False
        
        return True
    
    def migrate_all(self):
        """迁移所有表"""
        print(f"\n{'=' * 80}")
        print(f"Figma Deliver 数据库迁移工具")
        print(f"{'=' * 80}")
        print(f"源数据库: {self.source_config['user']}@{self.source_config['host']}:{self.source_config['port']}/{self.source_config['database']}")
        print(f"目标数据库: {self.target_config['user']}@{self.target_config['host']}:{self.target_config['port']}/{self.target_config['database']}")
        print(f"{'=' * 80}\n")
        
        # 连接数据库
        self.connect()
        
        # 禁用外键检查（允许删除和创建有依赖关系的表）
        self.disable_foreign_key_checks()
        
        # 获取所有表并按依赖关系排序
        tables = self.get_tables()
        sorted_tables = self.sort_tables_by_dependencies(tables)
        print(f"[{self._timestamp()}] 找到 {len(tables)} 个表")
        print(f"[{self._timestamp()}] 迁移顺序: {', '.join(sorted_tables)}\n")
        
        # 迁移每个表（使用排序后的表列表）
        success_count = 0
        failed_tables = []
        
        for i, table in enumerate(sorted_tables, 1):
            print(f"\n进度: [{i}/{len(sorted_tables)}]")
            if self.migrate_table(table):
                success_count += 1
            else:
                failed_tables.append(table)
        
        # 启用外键检查
        self.enable_foreign_key_checks()
        
        # 关闭连接
        self.close()
        
        # 显示迁移结果
        print(f"\n{'=' * 80}")
        print(f"迁移完成")
        print(f"{'=' * 80}")
        print(f"成功: {success_count}/{len(tables)} 个表")
        if failed_tables:
            print(f"失败: {len(failed_tables)} 个表: {', '.join(failed_tables)}")
        else:
            print(f"✓ 所有表迁移成功！")
        print(f"{'=' * 80}\n")
        
        return len(failed_tables) == 0
    
    @staticmethod
    def _timestamp():
        """获取当前时间戳"""
        return datetime.now().strftime("%Y-%m-%d %H:%M:%S")


def main():
    """主函数"""
    parser = argparse.ArgumentParser(description='Figma Deliver 数据库迁移工具')
    parser.add_argument('--dry-run', action='store_true', help='仅检查连接，不执行迁移')
    args = parser.parse_args()
    
    # 源数据库配置（本地数据库）
    source_config = {
        'host': '127.0.0.1',
        'port': 3306,
        'user': 'root',
        'password': 'mysql@zht2182',
        'database': 'figma_deliver'
    }
    
    # 目标数据库配置（远程数据库）
    target_config = {
        'host': '10.84.97.48',
        'port': 3306,
        'user': 'mysqlsiud',
        'password': 'mysql!@#456',
        'database': 'figma_deliver'
    }
    
    # 创建迁移工具实例
    migrator = DatabaseMigrator(source_config, target_config)
    
    if args.dry_run:
        print("执行连接测试...")
        migrator.connect()
        print("✓ 连接测试成功！")
        migrator.close()
        return
    
    # 执行迁移
    try:
        success = migrator.migrate_all()
        sys.exit(0 if success else 1)
    except KeyboardInterrupt:
        print("\n\n迁移已被用户中断")
        migrator.close()
        sys.exit(1)
    except Exception as e:
        print(f"\n迁移过程中发生错误: {e}")
        migrator.close()
        sys.exit(1)


if __name__ == '__main__':
    main()

