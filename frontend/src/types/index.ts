export interface User {
  id: number;
  username: string;
  email: string;
  role: 'admin' | 'player';
  created_at: string;
}

export interface Team {
  id: number;
  name: string;
  affiliation: string;
  owner_id: number;
  created_at: string;
}

export interface Arena {
	 frozen: boolean;
	 phase?: 'warmup' | 'running' | 'frozen' | 'finished';
  id: number;
  slug: string;
  name: string;
  description: string;
  game_type: string;
  max_players: number;
  max_ticks: number;
  config_json: Record<string, any>;
  is_active: boolean;
  created_at: string;
}

export interface AgentVersion {
  id: number;
  team_id: number;
  team_name?: string;
  arena_id: number;
  name: string;
  version: number;
  runtime: string;
  entrypoint: string;
  sha256: string;
  status: 'active' | 'disqualified' | 'pending_validation' | 'retired';
  failure_reason?: string;
  created_at: string;
}

export interface LadderEntry {
  id: number;
  arena_id: number;
  agent_version_id: number;
  agent_name: string;
  team_id: number;
  team_name: string;
  rating_mu: number;
  rating_sigma: number;
  display_rating: number;
  matches_played: number;
  wins: number;
  kills: number;
  survival_ticks_total: number;
  last_match_at: string | null;
  updated_at: string;
  status: string;
  win_rate: number;
  avg_survival_ticks: number;
}

export interface MatchParticipant {
  id: number;
  match_id: number;
  agent_version_id: number;
  agent_name: string;
  team_name: string;
  seat: number;
  rank_place: number;
  kills: number;
  survival_ticks: number;
  score: number;
  disqualified: boolean;
  disqualification_reason?: string;
  old_rating: number;
  new_rating: number;
  rating_delta: number;
}

export interface Match {
  id: number;
  arena_id: number;
  arena_name?: string;
  seed: number;
  status: 'scheduled' | 'running' | 'finished' | 'failed';
  ticks_played: number;
  winner_agent_id?: number;
  winner_name?: string;
  error_message?: string;
  execution_log?: string;
  started_at?: string;
  finished_at?: string;
  created_at: string;
  participants?: MatchParticipant[];
  replay_url?: string;
}

export interface AuditLog {
  id: number;
  user_id?: number;
  username?: string;
  action: string;
  target_type: string;
  target_id: string;
  ip_address: string;
  details_json: Record<string, any>;
  created_at: string;
}

export interface SystemStatus {
  status: string;
  database_healthy: boolean;
  arbiter_healthy: boolean;
  arbiter_path: string;
  matchmaker_active: boolean;
  running_matches: number;
  server_time: string;
}

// Replay frame types
export interface UnitState {
  vision?: number;
  id: number;
  seat: number;
  x: number;
  y: number;
  angle: number;
  hp: number;
  max_hp: number;
  alive: boolean;
  score?: number;
  kills: number;
}

export interface MobState {
  id: number;
  x: number;
  y: number;
  hp: number;
  alive: boolean;
}

export interface ProjectileState {
  x: number;
  y: number;
  vx?: number;
  vy?: number;
}

export interface ReplayTickFrame {
  kill_feed?: string[];
  tick: number;
  units: UnitState[];
  mobs?: MobState[];
  projectiles?: ProjectileState[];
  events?: string[];
  zone_radius?: number;
}

export interface ReplayData {
  event_format?: 'delta-v1';
  entity_format?: 'keyframe-delta-v1';
  walls?: { x: number; y: number; w: number; h: number }[];
  effective_config?: ReplayData['config'];
  arena?: { width: number; height: number; tick_hz: number };
  score_version?: string;
  engine_version?: string;
  config: {
    match_rules: {
      duration: number;
      walls: number;
      zone: boolean;
      show_vision?: boolean;
    };
  };
  frames?: ReplayTickFrame[];
  ticks?: number | ReplayTickFrame[];
  winner?: string;
  ranking?: any[];
}
