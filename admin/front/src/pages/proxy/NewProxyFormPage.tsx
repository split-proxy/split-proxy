import { ArrowLeftOutlined } from "@ant-design/icons";
import { Button, Card, Form, Input, Space, Typography, message } from "antd";
import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { getProxy, createProxy, updateProxy } from "@/shared/api/proxyApi";
import type { ProxyInput } from "@/shared/types";
import { useTranslation } from "react-i18next";

export function ProxyFormPage() {
  const { t, i18n } = useTranslation();
  const { id } = useParams();
  const navigate = useNavigate();
  const [form] = Form.useForm<ProxyInput>();
  const [loading, setLoading] = useState(Boolean(id));

  const isEdit = Boolean(id);

  useEffect(() => {
    if (!id) {
      setLoading(false);
      return;
    }

    const load = async () => {
      try {
        const proxy = await getProxy(Number(id));
        form.setFieldsValue({
          username: proxy.username,
          password: proxy.password,
        });
      } catch (e) {
        message.error(e instanceof Error ? e.message : t("newProxyFormPage.proxyNotLoaded"));
        navigate("/proxies", { replace: true });
      } finally {
        setLoading(false);
      }
    };

    void load();
  }, [id, form, navigate]);

  const submit = async () => {
    try {
      const values = await form.validateFields();

      if (id) {
        await updateProxy(Number(id), values);
        message.success(t("newProxyFormPage.proxyUpdated"));
      } else {
        await createProxy(values);
        message.success(t("newProxyFormPage.proxyCreated"));
      }

      navigate("/proxies");
    } catch (e) {
      if (e instanceof Error && e.message !== "validation") {
        message.error(e.message);
      }
    }
  };

  return (
    <div className="form-page">
      <div className="page-heading">
        <div>
          <Button
            type="text"
            icon={<ArrowLeftOutlined />}
            onClick={() => navigate("/proxies")}
            style={{ paddingLeft: 0, marginBottom: 8 }}
          >
            {t("newProxyFormPage.backToProxies")}
          </Button>
          <Typography.Title level={2} style={{ margin: 0 }}>
            {isEdit ? t("newProxyFormPage.proxyEditing") : t("newProxyFormPage.newProxy")}
          </Typography.Title>
        </div>
      </div>

      <Card loading={loading} className="proxy-form-card">
        <Form form={form} layout="vertical" onFinish={submit}>
          <Form.Item
            label={t("common.login")}
            name="username"
            rules={[{ required: true, message: t("common.enterLogin") }]}
          >
            <Input size="large" placeholder={t("newProxyFormPage.usernamePlaceholder")} />
          </Form.Item>

          <Form.Item
            label={t("common.password")}
            name="password"
            rules={[
              { min: 8, message: t("common.minPasswordSize") },
            ]}
          >
            <Input.Password size="large" placeholder="••••••••" />
          </Form.Item>

          <Space className="form-actions">
            <Button onClick={() => navigate("/proxies")}>
            {t("common.cancel")}
            </Button>
            <Button type="primary" htmlType="submit">
              {isEdit ? t("common.save") : t("common.create")}
            </Button>
          </Space>
        </Form>
      </Card>
    </div>
  );
}
