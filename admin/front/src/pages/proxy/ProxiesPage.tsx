import type { TableProps } from "antd";
import { DeleteOutlined, EditOutlined, PlusOutlined, CheckCircleOutlined, CloseCircleOutlined } from "@ant-design/icons";
import { Button, Space, Table, Tag, message } from "antd";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useEffect, useState } from "react";
import { DeleteProxyModal } from "./DeleteProxyModal";
import { getProxies, removeProxy } from "@/shared/api/proxyApi";
import { ProxyItem } from '@/shared/types';

export function ProxiesPage() {
  const navigate = useNavigate();
  const { t, i18n } = useTranslation();
  const [proxies, setProxies] = useState<ProxyItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [deletingProxy, setDeletingProxy] = useState<ProxyItem | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const loadProxies = async () => {
    setLoading(true);

    try {
      const response = await getProxies();
      setProxies(response.results);
    } catch {
      message.error(t("proxiesPage.proxiesNotLoaded"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadProxies();
  }, []);

  const remove = async () => {
    if (!deletingProxy) return;

    setDeleteLoading(true);
    try {
      await removeProxy(deletingProxy.id);
      message.success(t("proxiesPage.proxyDeleted"));
      setDeletingProxy(null);
      await loadProxies();
    } catch (e) {
      message.error(e instanceof Error ? e.message : t("proxiesPage.deletionError"));
    } finally {
      setDeleteLoading(false);
    }
  };

  const columns: TableProps<ProxyItem>["columns"] = [
    {
      title: t("proxiesPage.login"),
      dataIndex: "username",
      key: "username",
    },
    {
      title: t("common.updatedAt"),
      dataIndex: "updated_at",
      key: "updated_at",
      responsive: ["md"],
      render: (value: number) =>
        new Date(value * 1000).toLocaleString("ru-RU"),
    },
    {
      title: t("common.createdAt"),
      dataIndex: "created_at",
      key: "created_at",
      responsive: ["md"],
      render: (value: number) =>
        new Date(value * 1000).toLocaleString("ru-RU"),
    },
    {
      title: t("common.actions"),
      key: "actions",
      width: 120,
      render: (_, proxy) => (
        <Space>
          <Button
            aria-label={t("common.edit")}
            icon={<EditOutlined />}
            onClick={() => navigate(`/proxies/${proxy.id}/edit`)}
          />

          <Button
            aria-label={t("common.delete")}
            danger
            icon={<DeleteOutlined />}
            onClick={() => setDeletingProxy(proxy)}
          />
        </Space>
      ),
    },
  ];

  return (
    <div className="proxies-page">
      <div className="page-heading">
        <div>
          <h1>{t("proxiesPage.h1")}</h1>
        </div>

        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => navigate("/proxies/new")}
        >
          {t("proxiesPage.addProxy")}
        </Button>
      </div>

      <Table<ProxyItem>
        rowKey="id"
        columns={columns}
        dataSource={proxies}
        loading={loading}
        scroll={{ x: 480 }}
        pagination={{ pageSize: 10, hideOnSinglePage: true }}
      />

      <DeleteProxyModal
        open={deletingProxy !== null}
        proxy={deletingProxy}
        loading={deleteLoading}
        onCancel={() => setDeletingProxy(null)}
        onConfirm={remove}
      />
    </div>
  );
}
