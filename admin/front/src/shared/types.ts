export type User = {
  id: number;
  username: string;
  password: string;
  created_at: number;
  updated_at: number;
  is_superuser: string;
};

export type UserInput = {
  username: string;
  password: string;
};

export type ProxyItem = {
  id: number;
  username: string;
  password: string;
  created_at: number;
  updated_at: number;
};

export type ProxyInput = {
  username: string;
  password: string;
};

export type GroupItem = {
  id: number;
  group_name: string;
  workers: number;
  online: number;
  offline: number;
  created_at: number;
  updated_at: number;
};

export type GroupInput = {
  group_name: string;
}

export type WorkerItem = {
  guid: string;
  ip_address: string;
  online: boolean;
  group: number;
  created_at: number;
  updated_at: number;
}

export type WorkerInput = {
  group: number;
}

export type DomainRule = {
  pattern: string;
  group: number | null;
};

export type CidrRule = {
  cidr: string;
  worker_group: number | null;
};
