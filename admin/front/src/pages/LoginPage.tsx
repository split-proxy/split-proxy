import { LockOutlined, UserOutlined } from "@ant-design/icons";
import { Alert, Button, Card, Form, Input, Typography } from "antd";
import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth";
import { useTranslation } from "react-i18next";
import { LanguageSwitcher } from "../components/LanguageSwitcher";

type LoginForm = {
  email: string;
  password: string;
};

export function LoginPage() {
  const { t, i18n } = useTranslation();
  const { login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const from = (location.state as { from?: string } | null)?.from || "/admin/users";

  const onFinish = async (values: LoginForm) => {
    setError(null);
    setLoading(true);

    try {
      await login(values.email, values.password);
      navigate(from, { replace: true });
    } catch (e) {
      setError(e instanceof Error ? e.message : t("loginPage.loginError"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "grid",
        placeItems: "center",
        background: "#f5f5f5",
        padding: 20,
      }}
    >
      <Card style={{ width: "100%", maxWidth: 420 }} styles={{ body: { padding: 32 } }}>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 16,
            marginBottom: 24,
          }}
        >
          <Typography.Title level={2} style={{ marginTop: 0 }}>
            {t("auth.h1")}
          </Typography.Title>

          <LanguageSwitcher/>
        </div>

        {error && <Alert type="error" message={error} showIcon style={{ marginBottom: 16 }} />}

        <Form<LoginForm> layout="vertical" onFinish={onFinish} initialValues={{
          email: "",
          password: "",
        }}>
          <Form.Item
            label={t("common.login")}
            name="email"
            rules={[
              { required: true, message: t("common.enterLogin") },
            ]}
          >
            <Input prefix={<UserOutlined />} size="large" />
          </Form.Item>

          <Form.Item
            label={t("common.password")}
            name="password"
            rules={[{ required: true, message: t("common.enterPassword") }]}
          >
            <Input.Password prefix={<LockOutlined />} size="large" />
          </Form.Item>

          <Button type="primary" htmlType="submit" size="large" block loading={loading}>
            {t("common.enter")}
          </Button>
        </Form>
      </Card>
    </div>
  );
}
