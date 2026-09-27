import {
  User,
  Arena,
  LadderEntry,
  Match,
  AgentVersion,
  Team,
  AuditLog,
  SystemStatus,
  ReplayData,
  ArenaKitManifest
} from '../types';

import { normalizeReplay } from './replay';
const API_BASE = '/api/v1';

class ApiClient {
  private token: string | null = null;

  constructor() {
    this.token = localStorage.getItem('agentrix_token');
  }

  setToken(token: string | null) {
    this.token = token;
    if (token) {
      localStorage.setItem('agentrix_token', token);
    } else {
      localStorage.removeItem('agentrix_token');
    }
  }

  getToken(): string | null {
    return this.token;
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const requestToken = this.token;
    const headers: Record<string, string> = {
      Accept: 'application/json',
      ...(options.headers as Record<string, string>),
    };

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    if (!(options.body instanceof FormData) && !headers['Content-Type']) {
      headers['Content-Type'] = 'application/json';
    }

    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    });
	if (requestToken !== this.token) throw new Error('Session changed; refresh the view');

    if (!response.ok) {
      const errorBody = await response.json().catch(() => ({ error: response.statusText }));
      throw new Error(errorBody.error || `HTTP ${response.status}`);
    }

    return response.json();
  }

  // Auth
  async login(username: string, password: string): Promise<{ token: string; user: User }> {
    const data = await this.request<{ token: string; user: User }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    });
    this.setToken(data.token);
    return data;
  }

  logout() {
    this.setToken(null);
  }

  async getMe(): Promise<User> {
    return this.request<User>('/auth/me');
  }

  // System
  async getSystemStatus(): Promise<SystemStatus> {
    return this.request<SystemStatus>('/system/status');
  }

  // Arenas
  async getArenas(): Promise<Arena[]> {
    return this.request<Arena[]>('/arenas');
  }

  async getArena(id: number): Promise<Arena> {
    return this.request<Arena>(`/arenas/${id}`);
  }

  async getArenaKit(id: number): Promise<ArenaKitManifest> {
    return this.request<ArenaKitManifest>(`/arenas/${id}/kit`);
  }

  // Ladder
	async setArenaFreeze(id: number, frozen: boolean): Promise<{ frozen: boolean }> {
	  return this.request(`/arenas/${id}/freeze`, { method: 'POST', body: JSON.stringify({ frozen }) });
	}

	async setArenaPhase(id: number, phase: string): Promise<{ arena_id: number; phase: string; frozen: boolean }> {
	  return this.request(`/arenas/${id}/phase`, { method: 'POST', body: JSON.stringify({ phase }) });
	}

	async scheduleRound(arenaId: number, idempotencyKey: string, agentVersionIds: number[], seed?: number): Promise<{ id: number; total_matches: number; format_version: string }> {
	  return this.request(`/arenas/${arenaId}/rounds`, {
	    method: 'POST',
	    body: JSON.stringify({
	      idempotency_key: idempotencyKey,
	      agent_version_ids: agentVersionIds,
	      seed,
	    }),
	  });
	}

  async getLadder(arenaId: number): Promise<LadderEntry[]> {
    return this.request<LadderEntry[]>(`/arenas/${arenaId}/ladder`);
  }

  // Matches
  async getMatches(arenaId?: number, status?: string, beforeId?: number): Promise<Match[]> {
    const params = new URLSearchParams();
    if (arenaId) params.set('arena_id', arenaId.toString());
    if (status) params.set('status', status);
    if (beforeId) params.set('before_id', beforeId.toString());
    return this.request<Match[]>(`/matches?${params.toString()}`);
  }

  async getMatch(id: number): Promise<Match> {
    return this.request<Match>(`/matches/${id}`);
  }

  async triggerMatch(arenaId: number, agentVersionIds?: number[], seed?: number): Promise<{ message: string; match_id: number }> {
    return this.request<{ message: string; match_id: number }>('/matches/trigger', {
      method: 'POST',
      body: JSON.stringify({
        arena_id: arenaId,
        agent_version_ids: agentVersionIds,
        seed,
      }),
    });
  }

  async rerunMatch(id: number): Promise<{ message: string; new_match_id: number }> {
    return this.request<{ message: string; new_match_id: number }>(`/matches/${id}/rerun`, {
      method: 'POST',
    });
  }

  async getReplay(matchId: number): Promise<ReplayData> {
    return normalizeReplay(await this.request<ReplayData>(`/matches/${matchId}/replay`));
  }

  // Agents
  async getAgents(arenaId?: number): Promise<AgentVersion[]> {
    const params = new URLSearchParams();
    if (arenaId) params.set('arena_id', arenaId.toString());
    return this.request<AgentVersion[]>(`/agents?${params.toString()}`);
  }

  async uploadAgent(formData: FormData): Promise<{ message: string; agent_id: number }> {
    return this.request<{ message: string; agent_id: number }>('/agents/upload', {
      method: 'POST',
      body: formData,
    });
  }

  async disqualifyAgent(id: number, reason: string): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/agents/${id}/disqualify`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    });
  }

  async enableAgent(id: number): Promise<{ message: string }> {
    return this.request<{ message: string }>(`/agents/${id}/enable`, {
      method: 'POST',
    });
  }

  // Teams & Users
  async getTeams(): Promise<Team[]> {
    return this.request<Team[]>('/teams');
  }

  async getUsers(): Promise<User[]> {
    return this.request<User[]>('/users');
  }

  // Audit Logs
  async getAuditLogs(): Promise<AuditLog[]> {
    return this.request<AuditLog[]>('/audit-logs');
  }
}

export const api = new ApiClient();
