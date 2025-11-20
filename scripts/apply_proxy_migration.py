#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
应用代理配置字段迁移到数据库
"""

import pymysql
import sys

# 数据库配置（从 config copy.yaml）
DB_CONFIG = {
    'host': '10.84.97.48',
    'port': 3306,
    'user': 'mysqlsiud',
    'password': 'mysql!@#456',
    'database': 'figma_deliver',
    'charset': 'utf8mb4'
}

def execute_migration():
    """执行数据库迁移"""
    try:
        # 连接数据库
        print(f"正在连接数据库 {DB_CONFIG['host']}:{DB_CONFIG['port']}...")
        connection = pymysql.connect(**DB_CONFIG)
        cursor = connection.cursor()
        print("[OK] 数据库连接成功!\n")
        
        # 1. 添加 proxy_enabled 字段
        print("[1/3] 添加 proxy_enabled 字段...")
        try:
            cursor.execute("""
                ALTER TABLE users 
                ADD COLUMN proxy_enabled BOOLEAN DEFAULT FALSE 
                COMMENT '是否启用代理'
            """)
            print("  => proxy_enabled 字段添加成功")
        except pymysql.err.OperationalError as e:
            if "Duplicate column name" in str(e):
                print("  => proxy_enabled 字段已存在，跳过")
            else:
                raise
        
        # 2. 添加 proxy_url 字段
        print("\n[2/3] 添加 proxy_url 字段...")
        try:
            cursor.execute("""
                ALTER TABLE users 
                ADD COLUMN proxy_url VARCHAR(500) DEFAULT '' 
                COMMENT '代理地址，如 http://127.0.0.1:7890'
            """)
            print("  => proxy_url 字段添加成功")
        except pymysql.err.OperationalError as e:
            if "Duplicate column name" in str(e):
                print("  => proxy_url 字段已存在，跳过")
            else:
                raise
        
        # 3. 添加索引
        print("\n[3/3] 添加索引 idx_users_proxy_enabled...")
        try:
            cursor.execute("""
                CREATE INDEX idx_users_proxy_enabled ON users(proxy_enabled)
            """)
            print("  => 索引添加成功")
        except pymysql.err.OperationalError as e:
            if "Duplicate key name" in str(e):
                print("  => 索引已存在，跳过")
            else:
                raise
        
        # 提交更改
        connection.commit()
        
        # 4. 验证迁移结果
        print("\n[INFO] 验证迁移结果...")
        cursor.execute("""
            SELECT COLUMN_NAME, DATA_TYPE, COLUMN_DEFAULT, IS_NULLABLE, COLUMN_COMMENT 
            FROM INFORMATION_SCHEMA.COLUMNS 
            WHERE TABLE_SCHEMA = %s 
              AND TABLE_NAME = 'users' 
              AND COLUMN_NAME IN ('proxy_enabled', 'proxy_url')
            ORDER BY COLUMN_NAME
        """, (DB_CONFIG['database'],))
        
        results = cursor.fetchall()
        if results:
            print("\n字段信息:")
            print("-" * 100)
            print(f"{'字段名':<20} {'数据类型':<15} {'默认值':<15} {'允许NULL':<10} {'注释':<30}")
            print("-" * 100)
            for row in results:
                column_name = row[0]
                data_type = row[1]
                default = str(row[2]) if row[2] is not None else 'NULL'
                nullable = row[3]
                comment = row[4] if row[4] else ''
                print(f"{column_name:<20} {data_type:<15} {default:<15} {nullable:<10} {comment:<30}")
            print("-" * 100)
        
        print("\n[SUCCESS] 迁移完成！代理配置字段已成功添加到数据库。")
        
        # 关闭连接
        cursor.close()
        connection.close()
        
        return True
        
    except pymysql.Error as e:
        print(f"\n[ERROR] 数据库错误: {e}")
        return False
    except Exception as e:
        print(f"\n[ERROR] 发生错误: {e}")
        import traceback
        traceback.print_exc()
        return False

if __name__ == "__main__":
    print("=" * 100)
    print("代理配置字段数据库迁移工具")
    print("=" * 100)
    print()
    
    success = execute_migration()
    
    if success:
        sys.exit(0)
    else:
        sys.exit(1)

