# kkon

A command-line tool for operating connectors on a Kafka Connect cluster: creating, inspecting, changing and backing them up.

## Cluster objects

**Connector**:
A named, configured instance of a plugin running on the cluster; the main thing kkon operates on.
_Avoid_: job, pipeline

**Task**:
One unit of a connector's work, identified by the connector's name and a numeric id.
_Avoid_: worker (a worker is the Kafka Connect process that runs tasks)

**Plugin**:
A connector implementation installed on the cluster, identified by its class; either a source or a sink.
_Avoid_: connector class (when talking about the installed thing), driver

**Connector config**:
The flat set of string key/values that defines a connector, including its `name` and `connector.class`.
_Avoid_: settings, properties, payload

**State**:
The reported condition of a connector or task, such as RUNNING, PAUSED or FAILED.
_Avoid_: health, status (status is the report that carries states)

## Making connectors

**Template**:
A predefined connector config for a common plugin that the create wizard fills in.
_Avoid_: preset, example

**Connector submission**:
Turning a connector config into a new connector on the cluster, after validation.
_Avoid_: create request, POST

**Validation**:
The cluster's check of a connector config against its plugin, reporting problems per field.
_Avoid_: lint, verification

## Operating connectors

**Target**:
The connector, or the single task of a connector, that a command acts on.
_Avoid_: item, resource

**Lifecycle command**:
A command that changes a running connector without changing its config: pause, resume, restart or delete.
_Avoid_: action, operation

**Backup**:
A file holding the connector configs of selected connectors at one point in time.
_Avoid_: dump, snapshot, export

**Restore**:
Re-creating or overwriting connectors from a backup.
_Avoid_: import, reload

**Dry-run**:
Running a command to see what it would change, without changing anything on the cluster.
_Avoid_: preview, simulation
