import { Modal, Typography } from "antd";
import { ExclamationCircleOutlined } from "@ant-design/icons";
import type { WorkerItem } from "@/shared/types";
import { useTranslation } from "react-i18next";

type Props = {
  open: boolean;
  worker: WorkerItem | null;
  loading?: boolean;
  onCancel: () => void;
  onConfirm: () => Promise<void>;
};

export function DeleteWorkerModal({ open, worker, loading, onCancel, onConfirm }: Props) {
  const { t, i18n } = useTranslation();
  return (
    <Modal
      title={t("deleteWorkerModal.deleteWorker")}
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
          {t("deleteWorkerModal.deleteDescription", {name: worker?.guid})}
        </Typography.Paragraph>
      </div>
    </Modal>
  );
}
