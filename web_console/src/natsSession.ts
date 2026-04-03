/**
 * Session-scoped NATS over WebSocket using credentials from POST /v1/auth/login (`nats` block).
 * See docs/tech_specs/web_console.md (CYNAI.WEBCON.NatsWebSocketTransport).
 */
import {
  connect,
  JSONCodec,
  jwtAuthenticator,
  type NatsConnection,
} from "nats.ws";

export type LoginNatsCredentials = {
  url?: string;
  websocket_url?: string;
  jwt?: string;
  jwt_expires_at?: string;
};

const jc = JSONCodec<Record<string, unknown>>();

/** Connect to NATS via WebSocket using `websocket_url` and JWT from the gateway login response. */
export async function connectSessionNats(creds: LoginNatsCredentials): Promise<NatsConnection> {
  const ws = (creds.websocket_url || "").trim();
  const tok = (creds.jwt || "").trim();
  if (!ws || !tok) {
    throw new Error("nats: websocket_url and jwt are required");
  }
  return connect({
    servers: [ws],
    authenticator: jwtAuthenticator(tok),
  });
}

export { jc };
