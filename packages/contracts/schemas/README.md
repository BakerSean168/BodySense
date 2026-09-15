# Public stream schemas

`stream-event.v1.schema.json` is the canonical public `StreamEvent` wire authority.

The JSON Schema owns the static and runtime event shape for browser-facing consultation streaming and replay. Repository generation derives TypeScript declarations and the standalone runtime validator from this schema; Web consumers parse live and replayed events through the shared generated trust boundary before projecting them into feature state.

Go and Python internal runtime traffic has a separate private Proto/Protovalidate authority. Do not reuse the public JSON event schema as the Go↔Python command/event IDL, and do not reintroduce a handwritten parallel public parser.
