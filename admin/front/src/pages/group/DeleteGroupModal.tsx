import { Modal, Typography } from "antd";
import { ExclamationCircleOutlined } from "@ant-design/icons";
import type { GroupItem } from "@/shared/types";
import { useTranslation } from "react-i18next";

type Props = {
  open: boolean;
  group: GroupItem | null;
  loading?: boolean;
  onCancel: () => void;
  onConfirm: () => Promise<void>;
};

export function DeleteGroupModal({ open, group, loading, onCancel, onConfirm }: Props) {
  const { t, i18n } = useTranslation();
  return (
    <Modal
      title={t("deleteGroupModal.deleteGroup")}
      open={open}
      onCancel={onCancel}
      onOk={onConfirm}
      okText={t("common.delete")}
      cancelText={t("common.cancel")}
      okButtonProps={{ danger: true }}
      confirmLoading={loading}
      centered
      width="min(440px, calc(100vw - 32px))"
    >
      <div style={{ display: "flex", gap: 12, alignItems: "flex-start", paddingTop: 8 }}>
        <ExclamationCircleOutlined style={{ color: "#ff4d4f", fontSize: 22, marginTop: 2 }} />
        <Typography.Paragraph style={{ margin: 0 }}>
          {t("deleteGroupModal.deleteDescription", {name: group?.group_name})}
        </Typography.Paragraph>
      </div>
    </Modal>
  );
}
