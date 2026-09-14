import type { TableProps } from "antd";
import { DeleteOutlined, EditOutlined, PlusOutlined, CheckCircleOutlined, CloseCircleOutlined } from "@ant-design/icons";
import { Button, Space, Table, Tag, Select, message } from "antd";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useEffect, useState } from "react";
import { DeleteGroupModal } from "./DeleteGroupModal";
import { DeleteWorkerModal } from "./DeleteWorkerModal";
import { getGroups, removeGroup } from "@/shared/api/groupApi";
import { getWorkers, removeWorker, updateWorker } from "@/shared/api/workerApi";
import { GroupItem, WorkerItem } from '@/shared/types';

export function GroupsPage() {
  const navigate = useNavigate();
  const { t, i18n } = useTranslation();
  const [groups, setGroups] = useState<GroupItem[]>([]);
  const [workers, setWorkers] = useState<WorkerItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [deletingGroup, setDeletingGroup] = useState<GroupItem | null>(null);
  const [deletingWorker, setDeletingWorker] = useState<WorkerItem | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const loadGroups = async () => {
    setLoading(true);

    try {
      const response = await getGroups();
      setGroups(response.results);
    } catch {
      message.error(t("groupsPage.groupsNotLoaded"));
    } finally {
      setLoading(false);
    }
  };

  const loadWorkers = async () => {
    setLoading(true);

    try {
      const response = await getWorkers();
      setWorkers(response.results);
    } catch {
      message.error(t("groupsPage.workersNotLoaded"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadGroups();
    void loadWorkers();
  }, []);

  const removeGroupConfirmation = async () => {
    if (!deletingGroup) return;

    setDeleteLoading(true);
    try {
      await removeGroup(deletingGroup.id);
      message.success(t("groupsPage.groupDeleted"));
      setDeletingGroup(null);
      await loadGroups();
    } catch (e) {
      message.error(e instanceof Error ? e.message : t("groupsPage.deletionGroupError"));
    } finally {
      setDeleteLoading(false);
    }
  };

  const removeWorkerConfirmation = async () => {
    if (!deletingWorker) return;

    setDeleteLoading(true);
    try {
      await removeWorker(deletingWorker.guid);
      message.success(t("groupsPage.workerDeleted"));
      setDeletingWorker(null);
      await loadWorkers();
    } catch (e) {
      message.error(e instanceof Error ? e.message : t("groupsPage.deletionWorkerError"));
    } finally {
      setDeleteLoading(false);
    }
  };

  const groups_columns: TableProps<GroupItem>["columns"] = [
    {
      title: t("groupsPage.groupName"),
      dataIndex: "group_name",
      key: "group_name",
    },
    {
      title: t("groupsPage.workers"),
      dataIndex: "workers",
      key: "workers",
      align: "center",
    },
    {
      title: t("groupsPage.online"),
      dataIndex: "online",
      key: "online",
      align: "center",
      render: (value: number) => (
        <span style={{ color: "#52C41A" }}>
          {value}
        </span>
      ),
    },
    {
      title: t("groupsPage.offline"),
      dataIndex: "offline",
      key: "offline",
      align: "center",
      render: (value: number) => (
        <span style={{ color: "#ff4d4f" }}>
          {value}
        </span>
      ),
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
      align: "center",
      render: (_, group) => (
        <Space>
          <Button
            aria-label={t("common.edit")}
            icon={<EditOutlined />}
            onClick={() => navigate(`/groups/${group.id}/edit`)}
          />

          <Button
            aria-label={t("common.delete")}
            danger
            icon={<DeleteOutlined />}
            onClick={() => setDeletingGroup(group)}
          />
        </Space>
      ),
    },
  ];

  const worker_columns: TableProps<WorkerItem>["columns"] = [
    {
      dataIndex: "online",
      key: "online",
      align: "center",
      render: (online: boolean) => (
        <span
          style={{
            display: "inline-block",
            width: 8,
            height: 8,
            borderRadius: "50%",
            backgroundColor: online ? "#52C41A" : "#ff4d4f",
          }}
        />
      ),
    },
    {
      title: t("groupsPage.worgerGuid"),
      dataIndex: "guid",
      key: "guid",
    },
    {
      title: t("groupsPage.ip"),
      dataIndex: "ip_address",
      key: "ip_address",
    },
    {
      title: t("groupsPage.groupName"),
      dataIndex: "group",
      key: "group",
      render: (groupId: number | null, worker) => (
        <Select
          value={groupId ?? undefined}
          placeholder={t("groupsPage.selectGroup")}
          style={{ minWidth: 160 }}
          allowClear
          options={groups.map((group) => ({
            value: group.id,
            label: group.group_name,
          }))}
        onChange={async (value) => {
          try {
            await updateWorker(worker.guid, {
              group: value ?? null,
            });

            setWorkers((prev) =>
              prev.map((item) =>
                item.guid === worker.guid
                  ? { ...item, group: value ?? null }
                  : item,
              ),
            );

            message.success(t("groupsPage.workerGroupUpdated"));
          } catch (e) {
            message.error(
              e instanceof Error
                ? e.message
                : t("groupsPage.workerGroupUpdateError"),
            );
          }
        }}
        />
      ),
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
      align: "center",
      render: (_, worker) => (
        <Space>
          <Button
            aria-label={t("common.delete")}
            danger
            icon={<DeleteOutlined />}
            onClick={() => setDeletingWorker(worker)}
          />
        </Space>
      ),
    },
  ];

  return (
    <div className="groups-page">
      <div className="page-heading">
        <div>
          <h1>{t("groupsPage.h1")}</h1>
        </div>

        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => navigate("/groups/new")}
        >
          {t("groupsPage.addGroup")}
        </Button>
      </div>

      <Table<GroupItem>
        rowKey="id"
        columns={groups_columns}
        dataSource={groups}
        loading={loading}
        scroll={{ x: 480 }}
        pagination={{ pageSize: 10, hideOnSinglePage: true }}
      />

      <DeleteGroupModal
        open={deletingGroup !== null}
        group={deletingGroup}
        loading={deleteLoading}
        onCancel={() => setDeletingGroup(null)}
        onConfirm={removeGroupConfirmation}
      />
      <br/>

      <div className="page-heading">
        <div>
          <h1>{t("groupsPage.workers")}</h1>
        </div>
      </div>

      <Table<WorkerItem>
        rowKey="guid"
        columns={worker_columns}
        dataSource={workers}
        loading={loading}
        scroll={{ x: 480 }}
        pagination={{ pageSize: 10, hideOnSinglePage: true }}
      />

      <DeleteWorkerModal
        open={deletingWorker !== null}
        worker={deletingWorker}
        loading={deleteLoading}
        onCancel={() => setDeletingWorker(null)}
        onConfirm={removeWorkerConfirmation}
      />
    </div>
  );
}
