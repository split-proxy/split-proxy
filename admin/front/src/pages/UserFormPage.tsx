import { ArrowLeftOutlined } from "@ant-design/icons";
import { Button, Card, Form, Input, Space, Typography, message } from "antd";
import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { getUser, createUser, updateUser } from "@/shared/api/userApi";
import type { UserInput } from "@/shared/types";
import { useTranslation } from "react-i18next";

export function UserFormPage() {
  const { t, i18n } = useTranslation();
  const { id } = useParams();
  const navigate = useNavigate();
  const [form] = Form.useForm<UserInput>();
  const [loading, setLoading] = useState(Boolean(id));

  const isEdit = Boolean(id);

  useEffect(() => {
    if (!id) {
      setLoading(false);
      return;
    }

    const load = async () => {
      try {
        const user = await getUser(Number(id));
        form.setFieldsValue({
          username: user.username,
          password: user.password,
        });
      } catch (e) {
        message.error(e instanceof Error ? e.message : t("userFormPage.userNotLoaded"));
        navigate("/users", { replace: true });
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
        await updateUser(Number(id), values);
        message.success(t("userFormPage.userUpdated"));
      } else {
        await createUser(values);
        message.success(t("userFormPage.userCreated"));
      }

      navigate("/users");
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
            onClick={() => navigate("/users")}
            style={{ paddingLeft: 0, marginBottom: 8 }}
          >
            {t("userFormPage.backToUsers")}
          </Button>
          <Typography.Title level={2} style={{ margin: 0 }}>
            {isEdit ? t("userFormPage.userEditing") : t("userFormPage.newUser")}
          </Typography.Title>
        </div>
      </div>

      <Card loading={loading} className="user-form-card">
        <Form form={form} layout="vertical" onFinish={submit}>
          <Form.Item
            label={t("common.login")}
            name="username"
            rules={[{ required: true, message: t("common.enterLogin") }]}
          >
            <Input size="large" placeholder={t("userFormPage.loginPlaceholder")} />
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
            <Button onClick={() => navigate("/users")}>
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
