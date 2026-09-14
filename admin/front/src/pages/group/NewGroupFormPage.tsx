import { ArrowLeftOutlined } from "@ant-design/icons";
import { Button, Card, Form, Input, Space, Typography, message } from "antd";
import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { getGroup, createGroup, updateGroup } from "@/shared/api/groupApi";
import type { GroupInput } from "@/shared/types";
import { useTranslation } from "react-i18next";

export function GroupFormPage() {
  const { t, i18n } = useTranslation();
  const { id } = useParams();
  const navigate = useNavigate();
  const [form] = Form.useForm<GroupInput>();
  const [loading, setLoading] = useState(Boolean(id));

  const isEdit = Boolean(id);

  useEffect(() => {
    if (!id) {
      setLoading(false);
      return;
    }

    const load = async () => {
      try {
        const group = await getGroup(Number(id));
        form.setFieldsValue({group_name: group.group_name});
      } catch (e) {
        message.error(e instanceof Error ? e.message : t("newGroupFormPage.groupNotLoaded"));
        navigate("/groups", { replace: true });
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
        await updateGroup(Number(id), values);
        message.success(t("newGroupFormPage.groupUpdated"));
      } else {
        await createGroup(values);
        message.success(t("newGroupFormPage.groupCreated"));
      }

      navigate("/groups");
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
            onClick={() => navigate("/groups")}
            style={{ paddingLeft: 0, marginBottom: 8 }}
          >
            {t("newGroupFormPage.backToGroups")}
          </Button>
          <Typography.Title level={2} style={{ margin: 0 }}>
            {isEdit ? t("newGroupFormPage.groupEditing") : t("newGroupFormPage.newGroup")}
          </Typography.Title>
        </div>
      </div>

      <Card loading={loading} className="group-form-card">
        <Form form={form} layout="vertical" onFinish={submit}>
          <Form.Item
            label={t("newGroupFormPage.group_name")}
            name="group_name"
            rules={[{ required: true, message: t("common.enterLogin") }]}
          >
            <Input size="large" placeholder={t("newGroupFormPage.groupNamePlaceholder")} />
          </Form.Item>


          <Space className="form-actions">
            <Button onClick={() => navigate("/groups")}>
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
