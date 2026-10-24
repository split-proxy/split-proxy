import type { TableProps } from "antd";
import type { Key } from "react";
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import {
  Button,
  Space,
  Table,
  Select,
  Input,
  Modal,
  message,
} from "antd";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { useEffect, useState } from "react";
import { DeleteGroupModal } from "./DeleteGroupModal";
import { DeleteWorkerModal } from "./DeleteWorkerModal";
import { getGroups, removeGroup } from "@/shared/api/groupApi";
import {
  getWorkers,
  removeWorker,
  updateWorker,
} from "@/shared/api/workerApi";
import type { GroupItem, WorkerItem } from "@/shared/types";

type WorkerGroupFilter = number | "none" | "all";
type BulkGroupValue = number | "__none__";

const DEFAULT_PAGE_SIZE = 100;

export function GroupsPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();

  const [groups, setGroups] = useState<GroupItem[]>([]);
  const [workers, setWorkers] = useState<WorkerItem[]>([]);
  const [loading, setLoading] = useState(true);

  const [totalWorkers, setTotalWorkers] = useState(0);
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);

  const [deletingGroup, setDeletingGroup] =
    useState<GroupItem | null>(null);
  const [deletingWorker, setDeletingWorker] =
    useState<WorkerItem | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const [selectedWorkerGuids, setSelectedWorkerGuids] =
    useState<string[]>([]);
  const [workerSearch, setWorkerSearch] = useState("");
  const [workerGroupFilter, setWorkerGroupFilter] =
    useState<WorkerGroupFilter>("all");
  const [bulkGroup, setBulkGroup] =
    useState<BulkGroupValue | undefined>();
  const [bulkLoading, setBulkLoading] = useState(false);

  const loadGroups = async () => {
    try {
      const response = await getGroups();
      setGroups(response.results);
    } catch {
      message.error(t("groupsPage.groupsNotLoaded"));
    }
  };

  const loadWorkers = async () => {
    setLoading(true);

    try {
      const params: {
        guid?: string;
        group?: number | "none";
        page: number;
        page_size: number;
      } = {
        page: currentPage,
        page_size: pageSize,
      };

      const guid = workerSearch.trim();

      if (guid) {
        params.guid = guid;
      }

      if (workerGroupFilter !== "all") {
        params.group = workerGroupFilter;
      }

      const response = await getWorkers(params);

      setWorkers(response.results);
      setTotalWorkers(response.count);

      // Важно: не очищаем selectedWorkerGuids здесь.
      // Выбранные GUID должны сохраняться при переходе между страницами.
      // Удаление выделения выполняется явно при смене фильтров
      // и после успешного выполнения массовых операций.
    } catch {
      message.error(t("groupsPage.workersNotLoaded"));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadGroups();
  }, []);

  useEffect(() => {
    void loadWorkers();
  }, [currentPage, pageSize, workerSearch, workerGroupFilter]);

  const removeGroupConfirmation = async () => {
    if (!deletingGroup) return;

    setDeleteLoading(true);

    try {
      await removeGroup(deletingGroup.id);
      message.success(t("groupsPage.groupDeleted"));
      setDeletingGroup(null);

      await loadGroups();
      await loadWorkers();
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t("groupsPage.deletionGroupError"),
      );
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

      setSelectedWorkerGuids((prev) =>
        prev.filter((guid) => guid !== deletingWorker.guid),
      );

      await loadWorkers();
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : t("groupsPage.deletionWorkerError"),
      );
    } finally {
      setDeleteLoading(false);
    }
  };

  const workerRowSelection = {
    selectedRowKeys: selectedWorkerGuids,
    preserveSelectedRowKeys: true,
    onChange: (selectedRowKeys: Key[]) => {
      setSelectedWorkerGuids(selectedRowKeys.map(String));
    },
  };

  const bulkUpdateGroup = async () => {
    if (
      selectedWorkerGuids.length === 0 ||
      bulkGroup === undefined ||
      bulkLoading
    ) {
      return;
    }

    const guids = [...selectedWorkerGuids];
    const groupId: number | null =
      bulkGroup === "__none__" ? null : bulkGroup;

    setBulkLoading(true);

    try {
      const results = await Promise.allSettled(
        guids.map((guid) => updateWorker(guid, { group: groupId })),
      );

      const succeededGuids = guids.filter(
        (_, index) => results[index].status === "fulfilled",
      );

      const succeeded = succeededGuids.length;
      const failed = results.length - succeeded;

      setWorkers((prev) =>
        prev.map((worker) => {
          const index = guids.indexOf(worker.guid);

          if (
            index === -1 ||
            results[index].status !== "fulfilled"
          ) {
            return worker;
          }

          return { ...worker, group: groupId };
        }),
      );

      // Убираем из выделения только успешно обновлённые записи.
      // Если какие-то запросы упали, их GUID остаются выделенными.
      setSelectedWorkerGuids((prev) =>
        prev.filter((guid) => !succeededGuids.includes(guid)),
      );

      setBulkGroup(undefined);

      if (failed === 0) {
        message.success(
          t("groupsPage.bulkGroupUpdated", {
            count: succeeded,
          }),
        );
      } else {
        message.warning(
          t("groupsPage.bulkGroupPartial", {
            succeeded,
            failed,
          }),
        );
      }

      await loadWorkers();
    } finally {
      setBulkLoading(false);
    }
  };

  const bulkDeleteWorkers = () => {
    if (selectedWorkerGuids.length === 0 || bulkLoading) {
      return;
    }

    Modal.confirm({
      title: t("groupsPage.bulkDeleteTitle"),
      content: t("groupsPage.bulkDeleteConfirm", {
        count: selectedWorkerGuids.length,
      }),
      okText: t("common.delete"),
      okButtonProps: { danger: true },
      cancelText: t("common.cancel"),
      onOk: async () => {
        setBulkLoading(true);

        try {
          const guids = [...selectedWorkerGuids];

          const results = await Promise.allSettled(
            guids.map((guid) => removeWorker(guid)),
          );

          const deletedGuids = guids.filter(
            (_, index) => results[index].status === "fulfilled",
          );

          const succeeded = deletedGuids.length;
          const failed = results.length - succeeded;

          if (succeeded > 0) {
            setSelectedWorkerGuids((prev) =>
              prev.filter((guid) => !deletedGuids.includes(guid)),
            );
          }

          if (failed === 0) {
            message.success(
              t("groupsPage.bulkWorkersDeleted", {
                count: succeeded,
              }),
            );
          } else {
            message.warning(
              t("groupsPage.bulkDeletePartial", {
                succeeded,
                failed,
              }),
            );
          }

          const remaining = Math.max(0, totalWorkers - succeeded);
          const lastPage = Math.max(
            1,
            Math.ceil(remaining / pageSize),
          );

          if (currentPage > lastPage) {
            setCurrentPage(lastPage);
          } else {
            await loadWorkers();
          }
        } finally {
          setBulkLoading(false);
        }
      },
    });
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
        <span style={{ color: "#52C41A" }}>{value}</span>
      ),
    },
    {
      title: t("groupsPage.offline"),
      dataIndex: "offline",
      key: "offline",
      align: "center",
      render: (value: number) => (
        <span style={{ color: "#ff4d4f" }}>{value}</span>
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
      title: "",
      dataIndex: "online",
      key: "online",
      align: "center",
      width: 48,
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
            const newGroupId: number | null = value ?? null;

            try {
              await updateWorker(worker.guid, {
                group: newGroupId,
              });

              setWorkers((prev) =>
                prev.map((item) =>
                  item.guid === worker.guid
                    ? { ...item, group: newGroupId }
                    : item,
                ),
              );

              message.success(
                t("groupsPage.workerGroupUpdated"),
              );
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
      width: 80,
      align: "center",
      render: (_, worker) => (
        <Button
          aria-label={t("common.delete")}
          danger
          icon={<DeleteOutlined />}
          onClick={() => setDeletingWorker(worker)}
        />
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
        pagination={{
          pageSize: 10,
          hideOnSinglePage: true,
        }}
      />

      <DeleteGroupModal
        open={deletingGroup !== null}
        group={deletingGroup}
        loading={deleteLoading}
        onCancel={() => setDeletingGroup(null)}
        onConfirm={removeGroupConfirmation}
      />

      <div className="page-heading workers-heading">
        <div>
          <h1>{t("groupsPage.workers")}</h1>
        </div>
      </div>

      <Space
        wrap
        style={{ width: "100%", marginBottom: 16 }}
      >
        <Input
          allowClear
          placeholder={t("groupsPage.searchGuid")}
          value={workerSearch}
          onChange={(event) => {
            setWorkerSearch(event.target.value);
            setCurrentPage(1);
            setSelectedWorkerGuids([]);
          }}
          style={{ width: 240 }}
        />

        <Select<WorkerGroupFilter>
          value={workerGroupFilter}
          onChange={(value) => {
            setWorkerGroupFilter(value);
            setCurrentPage(1);
            setSelectedWorkerGuids([]);
          }}
          style={{ width: 220 }}
          options={[
            {
              value: "all",
              label: t("groupsPage.allGroups"),
            },
            {
              value: "none",
              label: t("groupsPage.noGroup"),
            },
            ...groups.map((group) => ({
              value: group.id,
              label: group.group_name,
            })),
          ]}
        />
      </Space>

      <Table<WorkerItem>
        rowKey="guid"
        rowSelection={workerRowSelection}
        columns={worker_columns}
        dataSource={workers}
        loading={loading || bulkLoading}
        scroll={{ x: 480 }}
        pagination={{
          current: currentPage,
          pageSize,
          total: totalWorkers,
          pageSizeOptions: [10, 20, 50, 100],
          showSizeChanger: true,
          hideOnSinglePage: true,
          showTotal: (total) =>
            t("groupsPage.workersTotal", { count: total }),
          onChange: (page, newPageSize) => {
            if (newPageSize !== pageSize) {
              setPageSize(newPageSize);
              setCurrentPage(1);
            } else {
              setCurrentPage(page);
            }
          },
        }}
      />

      {selectedWorkerGuids.length > 0 && (
        <div
          className="worker-bulk-footer"
          style={{
            position: "sticky",
            bottom: 0,
            zIndex: 10,
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            flexWrap: "wrap",
            gap: 12,
            padding: "16px 24px",
            marginTop: 16,
            background: "var(--ant-color-bg-container, #fff)",
            borderTop: "1px solid #f0f0f0",
            boxShadow: "0 -4px 12px rgba(0, 0, 0, 0.06)",
          }}
        >
          <span>
            {t("groupsPage.selectedWorkers", {
              count: selectedWorkerGuids.length,
            })}
          </span>

          <Space wrap>
            <Select<BulkGroupValue>
              placeholder={t("groupsPage.bulkSelectGroup")}
              value={bulkGroup}
              onChange={setBulkGroup}
              style={{ width: 220 }}
              options={[
                ...groups.map((group) => ({
                  value: group.id,
                  label: group.group_name,
                })),
                {
                  value: "__none__" as const,
                  label: t("groupsPage.noGroup"),
                },
              ]}
            />

            <Button
              type="primary"
              disabled={
                bulkGroup === undefined || bulkLoading
              }
              loading={bulkLoading}
              onClick={() => void bulkUpdateGroup()}
            >
              {t("groupsPage.bulkUpdateGroup")}
            </Button>

            <Button
              danger
              icon={<DeleteOutlined />}
              disabled={bulkLoading}
              loading={bulkLoading}
              onClick={bulkDeleteWorkers}
            >
              {t("groupsPage.bulkDelete")}
            </Button>

            <Button
              disabled={bulkLoading}
              onClick={() => setSelectedWorkerGuids([])}
            >
              {t("groupsPage.clearSelection")}
            </Button>
          </Space>
        </div>
      )}

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
