import { renderIcon } from '@/utils/index';
import {
  DashboardOutlined,
  SettingOutlined,
  TeamOutlined,
  SafetyCertificateOutlined,
  MenuOutlined,
  AuditOutlined,
  FileTextOutlined,
  CloudServerOutlined,
  ClusterOutlined,
  NodeIndexOutlined,
  VideoCameraOutlined,
  PlayCircleOutlined,
  ApiOutlined,
  ControlOutlined,
  DatabaseOutlined,
} from '@vicons/antd';

/**
 * 前端路由图标映射表。
 *
 * 后端菜单数据中的 iconName 字段（如 "DashboardOutlined"）通过此表映射为 Naive UI 可渲染的图标组件。
 * 如果图标名称不在表中，菜单将不显示图标（返回 null）。
 *
 * 新增图标步骤：
 *   1. 从 @vicons/antd (Ant Design Icons) 或 @vicons/ionicons5 导入所需图标
 *   2. 在下方对象中添加一行映射
 *   3. 后端菜单的 icon_name 字段填写对应的字符串
 */
export const constantRouterIcon: Record<string, () => any> = {
  // 仪表盘
  DashboardOutlined: renderIcon(DashboardOutlined),

  // 系统管理
  SettingOutlined: renderIcon(SettingOutlined),
  TeamOutlined: renderIcon(TeamOutlined),
  SafetyCertificateOutlined: renderIcon(SafetyCertificateOutlined),
  MenuOutlined: renderIcon(MenuOutlined),
  AuditOutlined: renderIcon(AuditOutlined),
  FileTextOutlined: renderIcon(FileTextOutlined),

  // 集群管理
  CloudServerOutlined: renderIcon(CloudServerOutlined),
  ClusterOutlined: renderIcon(ClusterOutlined),
  NodeIndexOutlined: renderIcon(NodeIndexOutlined),

  // 转码
  VideoCameraOutlined: renderIcon(VideoCameraOutlined),

  // 直播
  PlayCircleOutlined: renderIcon(PlayCircleOutlined),

  // 配置
  ApiOutlined: renderIcon(ApiOutlined),
  ControlOutlined: renderIcon(ControlOutlined),
  DatabaseOutlined: renderIcon(DatabaseOutlined),
};
