import {
  ApiOutlined,
  ArrowRightOutlined,
  BranchesOutlined,
  CheckCircleOutlined,
  CloudServerOutlined,
  DatabaseOutlined,
  ExclamationCircleOutlined,
  GroupOutlined,
  ReloadOutlined,
  SettingOutlined,
  SwapOutlined,
  TeamOutlined,
  WarningOutlined,
} from "@ant-design/icons";
import {
  Alert,
  Button,
  Card,
  Col,
  Empty,
  Progress,
  Row,
  Skeleton,
  Space,
  Statistic,
  Tag,
  Typography,
  message,
} from "antd";
import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";

import { getGroups } from "@/shared/api/groupApi";
import { getProxies } from "@/shared/api/proxyApi";
import { getListCidr, getListDomains, getDefaultRoute } from "@/shared/api/routingApi";
import { getUsers } from "@/shared/api/userApi";
import { getWorkers } from "@/shared/api/workerApi";
import { useConfig } from "@/shared/config/ConfigContext";
import type { GroupItem, WorkerItem } from "@/shared/types";

const ONLINE_COLOR = "#52c41a";
const OFFLINE_COLOR = "#ff4d4f";

function StatCard({
  title,
  value,
  icon,
  loading,
  suffix,
  onClick,
}: {
  title: string;
  value: number | string;
  icon: ReactNode;
  loading: boolean;
  suffix?: ReactNode;
  onClick?: () => void;
}) {
  return (
    <Card
      hoverable={Boolean(onClick)}
      onClick={onClick}
      style={{ height: "100%", cursor: onClick ? "pointer" : undefined }}
    >
      <Statistic
        title={title}
        value={loading ? undefined : value}
        suffix={suffix}
        prefix={icon}
      />
      {loading && <Skeleton active paragraph={false} title={{ width: "55%" }} />}
    </Card>
  );
}

export function HomePage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { config, loading: configLoading } = useConfig();

  const tx = (key: string, options?: Record<string, unknown>) =>
    t(`homePage.${key}`, options);

  const [groups, setGroups] = useState<GroupItem[]>([]);
  const [workers, setWorkers] = useState<WorkerItem[]>([]);
  const [workerCount, setWorkerCount] = useState(0);
  const [proxyCount, setProxyCount] = useState(0);
  const [adminCount, setAdminCount] = useState(0);
  const [domainCount, setDomainCount] = useState(0);
  const [cidrCount, setCidrCount] = useState(0);
  const [defaultRoute, setDefaultRoute] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);

  const loadDashboard = useCallback(async () => {
    setLoading(true);

    try {
      const [groupsResult, workersResult, proxiesResult, usersResult, domainsResult, cidrsResult, routeResult] =
        await Promise.allSettled([
          getGroups(),
          getWorkers(),
          getProxies(),
          getUsers(),
          getListDomains({ page: 1, pageSize: 1 }),
          getListCidr({ page: 1, pageSize: 1 }),
          getDefaultRoute(),
        ]);

      if (groupsResult.status === "fulfilled") setGroups(groupsResult.value.results);
      if (workersResult.status === "fulfilled") {
        setWorkers(workersResult.value.results);
        setWorkerCount(workersResult.value.count);
      }
      if (proxiesResult.status === "fulfilled") setProxyCount(proxiesResult.value.count);
      if (usersResult.status === "fulfilled") setAdminCount(usersResult.value.count);
      if (domainsResult.status === "fulfilled") setDomainCount(domainsResult.value.count);
      if (cidrsResult.status === "fulfilled") setCidrCount(cidrsResult.value.count);
      if (routeResult.status === "fulfilled") setDefaultRoute(routeResult.value.worker_group);

      const failed = [
        groupsResult,
        workersResult,
        proxiesResult,
        usersResult,
        domainsResult,
        cidrsResult,
        routeResult,
      ].some((result) => result.status === "rejected");

      if (failed) message.warning(tx("loadError"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadDashboard();
  }, [loadDashboard]);

  const onlineWorkers = workers.filter((worker) => worker.online).length;
  const offlineWorkers = workers.filter((worker) => !worker.online).length;
  const groupsWithProblems = groups.filter((group) => group.offline > 0 || group.online === 0);
  const workerAvailability = workerCount > 0 ? Math.round((onlineWorkers / workerCount) * 100) : 0;
  const defaultRouteGroup = groups.find((group) => group.id === defaultRoute);

  const recentWorkers = useMemo(
    () => [...workers].sort((a, b) => Number(b.online) - Number(a.online)).slice(0, 6),
    [workers],
  );

  const getGroupName = (id: number | null) =>
    groups.find((group) => group.id === id)?.group_name ?? "—";

  const pageLoading = loading || configLoading;

  return (
    <div className="home-page" style={{ maxWidth: 1400, margin: "0 auto" }}>
      <div className="page-heading">
        <div>
          <h1>{tx("title")}</h1>
          <div className="page-subtitle">{tx("subtitle")}</div>
        </div>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={() => void loadDashboard()}>
          {tx("refresh")}
        </Button>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard
            title={tx("workers")}
            value={onlineWorkers}
            suffix={`/ ${workerCount} ${tx("online")}`}
            icon={<CloudServerOutlined />}
            loading={pageLoading}
            onClick={() => navigate("/groups")}
          />
        </Col>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title={tx("groups")} value={groups.length} icon={<GroupOutlined />} loading={pageLoading} onClick={() => navigate("/groups")} />
        </Col>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title={tx("proxies")} value={proxyCount} icon={<SwapOutlined />} loading={pageLoading} onClick={() => navigate("/proxies")} />
        </Col>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title={tx("domains")} value={domainCount} icon={<BranchesOutlined />} loading={pageLoading} onClick={() => navigate("/splitting?tab=domains")} />
        </Col>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title={tx("cidrs")} value={cidrCount} icon={<ApiOutlined />} loading={pageLoading} onClick={() => navigate("/splitting?tab=cidrs")} />
        </Col>
        <Col xs={24} sm={12} lg={8} xl={4}>
          <StatCard title={tx("admins")} value={adminCount} icon={<TeamOutlined />} loading={pageLoading} onClick={() => navigate("/users")} />
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} xl={15}>
          <Card
            title={tx("workerGroups")}
            extra={<Button type="link" onClick={() => navigate("/groups")}>{tx("viewAll")}</Button>}
          >
            {groups.length === 0 && !pageLoading ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={tx("noGroups")} />
            ) : (
              <Space direction="vertical" size={14} style={{ width: "100%" }}>
                {pageLoading
                  ? Array.from({ length: 3 }).map((_, index) => <Skeleton key={index} active paragraph={{ rows: 1 }} />)
                  : groups.map((group) => {
                      const percentage = group.workers > 0 ? Math.round((group.online / group.workers) * 100) : 0;
                      const healthy = group.workers > 0 && group.offline === 0;

                      return (
                        <div key={group.id}>
                          <div style={{ display: "flex", justifyContent: "space-between", gap: 16, marginBottom: 6 }}>
                            <Space>
                              <Typography.Text strong>{group.group_name}</Typography.Text>
                              <Tag color={healthy ? "success" : "warning"}>{healthy ? tx("healthy") : tx("degraded")}</Tag>
                            </Space>
                            <Typography.Text type="secondary">
                              {group.online} / {group.workers} {tx("online")}
                            </Typography.Text>
                          </div>
                          <Progress percent={percentage} showInfo={false} status={healthy ? "normal" : "exception"} />
                        </div>
                      );
                    })}
              </Space>
            )}
          </Card>
        </Col>

        <Col xs={24} xl={9}>
          <Card title={tx("attention")}>
            {pageLoading ? (
              <Skeleton active paragraph={{ rows: 3 }} />
            ) : offlineWorkers > 0 || groupsWithProblems.length > 0 ? (
              <Space direction="vertical" size={12} style={{ width: "100%" }}>
                {offlineWorkers > 0 && (
                  <Alert
                    type="error"
                    showIcon
                    icon={<ExclamationCircleOutlined />}
                    message={tx("workersOffline", {"count": offlineWorkers})}
                    action={<Button type="link" onClick={() => navigate("/groups")}>{tx("viewAll")}</Button>}
                  />
                )}
                {groupsWithProblems.slice(0, 3).map((group) => (
                  <Alert
                    key={group.id}
                    type={group.online === 0 ? "error" : "warning"}
                    showIcon
                    icon={<WarningOutlined />}
                    message={tx("emptyGroup", {"name": group.group_name})}
                  />
                ))}
              </Space>
            ) : (
              <Alert type="success" showIcon icon={<CheckCircleOutlined />} message={tx("noProblems")} />
            )}
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} md={12} xl={8}>
          <Card title={tx("routing")}>
            <Space direction="vertical" size={16} style={{ width: "100%" }}>
              <div>
                <Typography.Text type="secondary">{tx("defaultRoute")}</Typography.Text>
                <div style={{ marginTop: 4 }}>
                  <Typography.Text strong style={{ fontSize: 18 }}>
                    {defaultRouteGroup?.group_name ?? config?.default_group_name ?? (defaultRoute === null ? "—" : String(defaultRoute))}
                  </Typography.Text>
                </div>
              </div>
              <Row gutter={12}>
                <Col span={12}>
                  <Statistic title={tx("domainRules")} value={domainCount} prefix={<BranchesOutlined />} />
                </Col>
                <Col span={12}>
                  <Statistic title={tx("cidrRules")} value={cidrCount} prefix={<DatabaseOutlined />} />
                </Col>
              </Row>
              {defaultRoute === null && !pageLoading && (
                <Typography.Text type="warning">{tx("defaultRouteUnavailable")}</Typography.Text>
              )}
              <Button type="link" icon={<ArrowRightOutlined />} onClick={() => navigate("/splitting")} style={{ paddingInline: 0 }}>
                {tx("viewAll")}
              </Button>
            </Space>
          </Card>
        </Col>

        <Col xs={24} md={12} xl={8}>
          <Card title={tx("quickActions")}>
            <Space direction="vertical" size={10} style={{ width: "100%" }}>
              <Button block icon={<GroupOutlined />} onClick={() => navigate("/groups/new")}>{tx("addGroup")}</Button>
              <Button block icon={<SwapOutlined />} onClick={() => navigate("/proxies/new")}>{tx("addProxy")}</Button>
              <Button block icon={<BranchesOutlined />} onClick={() => navigate("/splitting?tab=domains")}>{tx("addDomain")}</Button>
              <Button block icon={<ApiOutlined />} onClick={() => navigate("/splitting?tab=cidrs")}>{tx("addCidr")}</Button>
            </Space>
          </Card>
        </Col>

        <Col xs={24} xl={8}>
          <Card
            title={tx("workers")}
            extra={<Button type="link" onClick={() => navigate("/groups")}>{tx("viewAll")}</Button>}
          >
            {pageLoading ? (
              <Skeleton active paragraph={{ rows: 5 }} />
            ) : recentWorkers.length === 0 ? (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={tx("noWorkers")} />
            ) : (
              <Space direction="vertical" size={11} style={{ width: "100%" }}>
                {recentWorkers.map((worker) => (
                  <div key={worker.guid} style={{ display: "flex", alignItems: "center", gap: 10 }}>
                    <span
                      style={{
                        width: 8,
                        height: 8,
                        flex: "0 0 auto",
                        borderRadius: "50%",
                        background: worker.online ? ONLINE_COLOR : OFFLINE_COLOR,
                      }}
                    />
                    <div style={{ minWidth: 0, flex: 1 }}>
                      <Typography.Text ellipsis style={{ display: "block" }}>
                        {worker.ip_address}
                      </Typography.Text>
                      <Typography.Text type="secondary" ellipsis style={{ display: "block", fontSize: 12 }}>
                        {getGroupName(worker.group)}
                      </Typography.Text>
                    </div>
                    <Tag color={worker.online ? "success" : "error"}>
                      {worker.online ? tx("online") : tx("offline")}
                    </Tag>
                  </div>
                ))}
              </Space>
            )}
          </Card>
        </Col>
      </Row>

      {!pageLoading && workerCount > 0 && (
        <Card style={{ marginTop: 16 }}>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16, flexWrap: "wrap" }}>
            <Space>
              <SettingOutlined />
              <Typography.Text strong>{tx("workers")}</Typography.Text>
              <Typography.Text type="secondary">
                {onlineWorkers} {tx("online")}, {offlineWorkers} {tx("offline")}
              </Typography.Text>
            </Space>
            <Progress type="circle" size={48} percent={workerAvailability} />
          </div>
        </Card>
      )}
    </div>
  );
}
