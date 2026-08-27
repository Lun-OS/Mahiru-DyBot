// Account types
export interface Account {
  id: string;
  name: string;
  uid: string;
  state: AccountState;
  onebot_token: string;
  created_at: string;
  last_active: string;
  error?: string;
}

export type AccountState = 'stopped' | 'starting' | 'qr_pending' | 'online' | 'error';

// Auth types
export interface AuthResponse {
  token: string;
  expires_at: number;
}

export interface LoginRequest {
  password: string;
}

export interface SetupRequest {
  password: string;
}

// Settings types
export interface Settings {
  listen_addr: string;
  onebot_access_token: string;
  screenshot_max_fps: number;
  jpeg_quality: number;
  reverse_ws: ReverseWS[];
}

export interface ReverseWS {
  url: string;
  access_token: string;
  enabled: boolean;
}

// API Response types
export interface ApiResponse<T = unknown> {
  status: 'ok' | 'failed';
  retcode: number;
  data?: T;
  message?: string;
  wording?: string;
}

// Account Info
export interface AccountInfo {
  id: string;
  name: string;
  uid: string;
  state: AccountState;
  nickname: string;
  avatar: string;
  viewport: { width: number; height: number };
  viewport_width: number;
  viewport_height: number;
  custom_ua: string;
  actual_ua: string;
  actual_viewport_width: number;
  actual_viewport_height: number;
  sdk_ready: boolean;
  mod_id: number;
}

// SSE Event types
export interface SSEEvent {
  type: 'account_status' | 'sdk_ready' | 'new_message' | 'settings_changed';
  data: Record<string, unknown>;
}

// Dashboard stats
export interface DashboardStats {
  accounts_total: number;
  accounts_online: number;
  uptime: number;
  version: string;
}

// Login info (OneBot v11)
export interface LoginInfo {
  user_id: number;
  nickname: string;
  sec_uid?: string;
  unique_id?: string;
  short_id?: string;
  avatar?: string;
  signature?: string;
  gender?: number;
  sdk_ready?: boolean;
  mod_id?: number;
  connection_status?: string;
}

// Conversation
export interface Conversation {
  id: string;
  short_id: string;
  type: number;
  to_uid: string;
  name: string;
  nickname: string;
  avatar: string;
  unread: number;
}

// Message
export interface Message {
  server_id: string;
  client_id: string;
  sender: string;
  type: number;
  text: string;
  content: string;
  created_at: number;
  is_from_me: boolean;
}

// User Info
export interface UserInfo {
  uid: string;
  sec_uid: string;
  nickname: string;
  unique_id: string;
  short_id: string;
  signature: string;
  avatar_thumb: string;
  avatar_small: string;
  follow_status: number;
  follower_status: number;
  verification_type: number;
  custom_verify: string;
  enterprise_verify_reason: string;
  store_region: string;
}

// Group
export interface Group {
  group_id: number;
  group_name: string;
  member_count: number;
}

// Group Member
export interface GroupMember {
  uid: string;
  sec_uid: string;
  nickname: string;
  role: number;
}

// Adapter types
export type AdapterType = 'forward_ws' | 'reverse_ws' | 'http_server';

export interface Adapter {
  id: string;
  type: AdapterType;
  name: string;
  url: string;
  token: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}
