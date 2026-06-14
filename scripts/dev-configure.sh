#!/usr/bin/env sh
# Writes the local dev config for kkon and seeds sample connectors.
# Run this after `make dev-up` instead of going through interactive `kkon configure`.

BASE_URL="http://localhost:8083"
CONFIG_DIR="$HOME/.config/kkon"
CONFIG_FILE="$CONFIG_DIR/config.yaml"

# ── Write kkon config ──────────────────────────────────────────────────────────
mkdir -p "$CONFIG_DIR"
cat > "$CONFIG_FILE" <<'EOF'
kafkaConnect:
  url: http://localhost:8083
  username: ""
  password: ""
EOF
echo "Config written to $CONFIG_FILE"

# ── Wait for Kafka Connect to be ready ───────────────────────────────────────
echo "Waiting for Kafka Connect at $BASE_URL ..."
RETRIES=30
until curl -sf "$BASE_URL/connector-plugins" | grep -q "class" 2>/dev/null; do
  RETRIES=$((RETRIES - 1))
  if [ "$RETRIES" -eq 0 ]; then
    echo "ERROR: Kafka Connect did not become ready in time." >&2
    exit 1
  fi
  printf "."
  sleep 3
done
echo " ready."

# ── Helper to create a connector (idempotent — skips if already exists) ──────
create_connector() {
  NAME="$1"
  PAYLOAD="$2"
  if curl -sf "$BASE_URL/connectors/$NAME" > /dev/null 2>&1; then
    echo "Connector '$NAME' already exists, skipping."
  else
    RESPONSE=$(curl -s -o /tmp/connector_response.txt -w "%{http_code}" \
      -X POST "$BASE_URL/connectors" \
      -H "Content-Type: application/json" \
      -d "$PAYLOAD")
    if [ "$RESPONSE" = "201" ]; then
      echo "Created connector '$NAME'."
    else
      echo "ERROR creating '$NAME' (HTTP $RESPONSE):" >&2
      cat /tmp/connector_response.txt >&2
      echo "" >&2
    fi
  fi
}

# ── Sample connectors (MirrorMaker 2 — the only plugins in cp-kafka-connect) ──

# 1. Heartbeat: minimal connector, should reach RUNNING quickly
create_connector "dev-heartbeat" '{
  "name": "dev-heartbeat",
  "config": {
    "connector.class": "org.apache.kafka.connect.mirror.MirrorHeartbeatConnector",
    "tasks.max": "1",
    "source.cluster.alias": "source",
    "target.cluster.alias": "target",
    "source.cluster.bootstrap.servers": "kafka:9092",
    "target.cluster.bootstrap.servers": "kafka:9092",
    "emit.heartbeats.interval.seconds": "5"
  }
}'

# 2. Source mirror: replicates topics from the local cluster to itself
create_connector "dev-mirror-source" '{
  "name": "dev-mirror-source",
  "config": {
    "connector.class": "org.apache.kafka.connect.mirror.MirrorSourceConnector",
    "tasks.max": "1",
    "source.cluster.alias": "source",
    "target.cluster.alias": "target",
    "source.cluster.bootstrap.servers": "kafka:9092",
    "target.cluster.bootstrap.servers": "kafka:9092",
    "replication.factor": "1"
  }
}'

# 3. Broken: points at a non-existent broker — enters FAILED state for health-check UX testing
create_connector "dev-broken" '{
  "name": "dev-broken",
  "config": {
    "connector.class": "org.apache.kafka.connect.mirror.MirrorHeartbeatConnector",
    "tasks.max": "1",
    "source.cluster.alias": "source",
    "target.cluster.alias": "target",
    "source.cluster.bootstrap.servers": "nonexistent-broker:9092",
    "target.cluster.bootstrap.servers": "nonexistent-broker:9092",
    "emit.heartbeats.interval.seconds": "5"
  }
}'

echo "Done. Run: kkon connector list"
