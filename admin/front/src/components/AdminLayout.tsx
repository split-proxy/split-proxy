import {
  LogoutOutlined,
  MenuOutlined,
  TeamOutlined,
  DashboardOutlined,
  BranchesOutlined,
  SwapOutlined ,
  GroupOutlined
} from "@ant-design/icons";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { Button, Drawer, Layout, Menu, Typography } from "antd";
import { useState } from "react";
import { Link, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth";
import { useTranslation } from "react-i18next";

const { Header, Sider, Content } = Layout;

export function AdminLayout() {
  const { t, i18n } = useTranslation();
  const { user, logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [mobileOpen, setMobileOpen] = useState(false);

  const handleLogout = () => {
    logout();
    navigate("/login", { replace: true });
  };

  const handleMenuClick = () => setMobileOpen(false);

  const menuItems = [
    {
      key: "/",
      icon: <DashboardOutlined />,
      label: <Link to="/">{t("adminLayout.main")}</Link>,
    },
    {
      key: "/groups",
      icon: <GroupOutlined />,
      label: <Link to="/groups">{t("adminLayout.groups")}</Link>,
    },
    {
      key: "/splitting/",
      icon: <BranchesOutlined />,
      label: <Link to="/splitting/">{t("adminLayout.navigate")}</Link>,
    },
    {
      key: "/proxies",
      icon: <SwapOutlined />,
      label: <Link to="/proxies">{t("adminLayout.proxyAccounts")}</Link>,
    },
    {
      key: "/users",
      icon: <TeamOutlined />,
      label: <Link to="/users">{t("adminLayout.admins")}</Link>,
    },
  ];

  const menu = (
    <Menu
      theme="dark"
      mode="inline"
      selectedKeys={[location.pathname]}
      items={menuItems}
      onClick={handleMenuClick}
      style={{ borderInlineEnd: 0 }}
    />
  );

  return (
    <Layout className="admin-layout">
      <Sider className="desktop-sider" width={240}>
        <div className="brand">Split Proxy</div>
        {menu}
      </Sider>

      <Drawer
        className="mobile-drawer"
        placement="left"
        size={280}
        open={mobileOpen}
        onClose={() => setMobileOpen(false)}
        closable={false}
        styles={{
          body: { padding: 0, background: "#001529" },
        }}
      >
        <div className="brand">JWT Admin</div>
        {menu}
      </Drawer>

      <Layout className="admin-main">
        <Header className="admin-header">
          <div className="header-left">
            <Button
              className="mobile-menu-button"
              type="text"
              icon={<MenuOutlined />}
              aria-label="{t(adminLayout.openMenu)}"
              onClick={() => setMobileOpen(true)}
            />
            <Typography.Text strong>{t("adminLayout.controlPanel")}</Typography.Text>
          </div>

          <div className="header-right">
            <Typography.Text className="current-user">{user?.email}</Typography.Text>
            <LanguageSwitcher/>
            <Button
              icon={<LogoutOutlined />}
              onClick={handleLogout}
              className="logout-button"
            >
              {t("adminLayout.exit")}
            </Button>
          </div>
        </Header>

        <Content className="admin-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
