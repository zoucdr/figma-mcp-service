using UnityEngine;
using UnityEngine.UI;

namespace Figma2UGUI
{
    /// <summary>
    /// 角度渐变效果
    /// </summary>
    [AddComponentMenu("UI/Effects/Angular Gradient")]
    public class AngularGradient : BaseGradient
    {
        [SerializeField]
        private Vector2 m_Center = new Vector2(0.5f, 0.5f);

        [SerializeField]
        private Vector2 m_Direction1 = Vector2.right;

        [SerializeField]
        private Vector2 m_Direction2 = Vector2.up;

        /// <summary>
        /// 渐变中心点
        /// </summary>
        public Vector2 Center
        {
            get { return m_Center; }
            set
            {
                m_Center = value;
                if (graphic != null)
                {
                    graphic.SetVerticesDirty();
                }
            }
        }

        /// <summary>
        /// 方向1
        /// </summary>
        public Vector2 Direction1
        {
            get { return m_Direction1; }
            set
            {
                m_Direction1 = value.normalized;
                if (graphic != null)
                {
                    graphic.SetVerticesDirty();
                }
            }
        }

        /// <summary>
        /// 方向2
        /// </summary>
        public Vector2 Direction2
        {
            get { return m_Direction2; }
            set
            {
                m_Direction2 = value.normalized;
                if (graphic != null)
                {
                    graphic.SetVerticesDirty();
                }
            }
        }

        /// <summary>
        /// 计算顶点在角度渐变中的位置 (0-1)
        /// </summary>
        protected override float CalculateGradientPosition(Vector3 vertexPosition)
        {
            // 将顶点位置转换为局部空间中的2D坐标
            Vector2 point = new Vector2(vertexPosition.x, vertexPosition.y);

            // 计算顶点到中心的向量
            Vector2 toPoint = point - m_Center;

            if (toPoint.sqrMagnitude < 0.0001f)
            {
                return 0;
            }

            // 计算在两个方向上的投影
            float progress1 = CalcProgress(toPoint, m_Direction1, true);
            float progress2 = CalcProgress(toPoint, m_Direction2, true);

            // 计算角度
            float angle = Vector2.SignedAngle(Vector2.right, new Vector2(progress1, progress2));

            // 确保角度在0-360范围内
            if (angle < 0)
            {
                angle += 360;
            }

            // 转换为0-1范围
            float position = angle / 360f;

            return position;
        }

        /// <summary>
        /// 计算向量在指定方向上的投影比例
        /// </summary>
        private float CalcProgress(Vector2 point, Vector2 direction, bool sign = false)
        {
            if (direction.sqrMagnitude < 0.0001f)
            {
                return 0;
            }

            float dot = Vector2.Dot(point, direction);

            if (sign)
            {
                return Mathf.Sign(dot) * ((dot / direction.sqrMagnitude) * direction).magnitude / direction.magnitude;
            }
            else
            {
                return ((dot / direction.sqrMagnitude) * direction).magnitude / direction.magnitude;
            }
        }
    }
}
