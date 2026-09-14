from src.generated.runtimeproto.buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class StartTurnCommand(_message.Message):
    __slots__ = ("thread_id", "run_id", "conversation_id", "user_id", "configuration_id", "input", "business_context")
    THREAD_ID_FIELD_NUMBER: _ClassVar[int]
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    CONFIGURATION_ID_FIELD_NUMBER: _ClassVar[int]
    INPUT_FIELD_NUMBER: _ClassVar[int]
    BUSINESS_CONTEXT_FIELD_NUMBER: _ClassVar[int]
    thread_id: str
    run_id: str
    conversation_id: str
    user_id: str
    configuration_id: str
    input: ConsultationUserInput
    business_context: ConsultationBusinessContext
    def __init__(self, thread_id: _Optional[str] = ..., run_id: _Optional[str] = ..., conversation_id: _Optional[str] = ..., user_id: _Optional[str] = ..., configuration_id: _Optional[str] = ..., input: _Optional[_Union[ConsultationUserInput, _Mapping]] = ..., business_context: _Optional[_Union[ConsultationBusinessContext, _Mapping]] = ...) -> None: ...

class ResumeInterruptCommand(_message.Message):
    __slots__ = ("thread_id", "run_id", "conversation_id", "user_id", "configuration_id", "interrupt_id", "answer", "business_context")
    THREAD_ID_FIELD_NUMBER: _ClassVar[int]
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    CONFIGURATION_ID_FIELD_NUMBER: _ClassVar[int]
    INTERRUPT_ID_FIELD_NUMBER: _ClassVar[int]
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    BUSINESS_CONTEXT_FIELD_NUMBER: _ClassVar[int]
    thread_id: str
    run_id: str
    conversation_id: str
    user_id: str
    configuration_id: str
    interrupt_id: str
    answer: _struct_pb2.Struct
    business_context: ConsultationBusinessContext
    def __init__(self, thread_id: _Optional[str] = ..., run_id: _Optional[str] = ..., conversation_id: _Optional[str] = ..., user_id: _Optional[str] = ..., configuration_id: _Optional[str] = ..., interrupt_id: _Optional[str] = ..., answer: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., business_context: _Optional[_Union[ConsultationBusinessContext, _Mapping]] = ...) -> None: ...

class ConsultationImageRef(_message.Message):
    __slots__ = ("upload_id", "mime_type", "data_url")
    UPLOAD_ID_FIELD_NUMBER: _ClassVar[int]
    MIME_TYPE_FIELD_NUMBER: _ClassVar[int]
    DATA_URL_FIELD_NUMBER: _ClassVar[int]
    upload_id: str
    mime_type: str
    data_url: str
    def __init__(self, upload_id: _Optional[str] = ..., mime_type: _Optional[str] = ..., data_url: _Optional[str] = ...) -> None: ...

class ConsultationUserInput(_message.Message):
    __slots__ = ("type", "text", "images")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    type: str
    text: str
    images: _containers.RepeatedCompositeFieldContainer[ConsultationImageRef]
    def __init__(self, type: _Optional[str] = ..., text: _Optional[str] = ..., images: _Optional[_Iterable[_Union[ConsultationImageRef, _Mapping]]] = ...) -> None: ...

class ConsultationRuntimeState(_message.Message):
    __slots__ = ("phase", "extracted_info")
    PHASE_FIELD_NUMBER: _ClassVar[int]
    EXTRACTED_INFO_FIELD_NUMBER: _ClassVar[int]
    phase: str
    extracted_info: _containers.RepeatedCompositeFieldContainer[_struct_pb2.Struct]
    def __init__(self, phase: _Optional[str] = ..., extracted_info: _Optional[_Iterable[_Union[_struct_pb2.Struct, _Mapping]]] = ...) -> None: ...

class ConsultationSpatialContext(_message.Message):
    __slots__ = ("body_region_id", "body_region_label", "anatomy_id", "anatomy_name")
    BODY_REGION_ID_FIELD_NUMBER: _ClassVar[int]
    BODY_REGION_LABEL_FIELD_NUMBER: _ClassVar[int]
    ANATOMY_ID_FIELD_NUMBER: _ClassVar[int]
    ANATOMY_NAME_FIELD_NUMBER: _ClassVar[int]
    body_region_id: str
    body_region_label: str
    anatomy_id: str
    anatomy_name: str
    def __init__(self, body_region_id: _Optional[str] = ..., body_region_label: _Optional[str] = ..., anatomy_id: _Optional[str] = ..., anatomy_name: _Optional[str] = ...) -> None: ...

class ConsultationBusinessContext(_message.Message):
    __slots__ = ("profile", "body_state", "runtime_state", "relevant_history", "current_diagnosis", "current_treatment", "recent_outcomes", "spatial_context", "posture_analysis")
    PROFILE_FIELD_NUMBER: _ClassVar[int]
    BODY_STATE_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_STATE_FIELD_NUMBER: _ClassVar[int]
    RELEVANT_HISTORY_FIELD_NUMBER: _ClassVar[int]
    CURRENT_DIAGNOSIS_FIELD_NUMBER: _ClassVar[int]
    CURRENT_TREATMENT_FIELD_NUMBER: _ClassVar[int]
    RECENT_OUTCOMES_FIELD_NUMBER: _ClassVar[int]
    SPATIAL_CONTEXT_FIELD_NUMBER: _ClassVar[int]
    POSTURE_ANALYSIS_FIELD_NUMBER: _ClassVar[int]
    profile: _struct_pb2.Struct
    body_state: _struct_pb2.Struct
    runtime_state: ConsultationRuntimeState
    relevant_history: _containers.RepeatedCompositeFieldContainer[_struct_pb2.Struct]
    current_diagnosis: _struct_pb2.Struct
    current_treatment: _struct_pb2.Struct
    recent_outcomes: _containers.RepeatedCompositeFieldContainer[_struct_pb2.Struct]
    spatial_context: ConsultationSpatialContext
    posture_analysis: _struct_pb2.Struct
    def __init__(self, profile: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., body_state: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., runtime_state: _Optional[_Union[ConsultationRuntimeState, _Mapping]] = ..., relevant_history: _Optional[_Iterable[_Union[_struct_pb2.Struct, _Mapping]]] = ..., current_diagnosis: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., current_treatment: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., recent_outcomes: _Optional[_Iterable[_Union[_struct_pb2.Struct, _Mapping]]] = ..., spatial_context: _Optional[_Union[ConsultationSpatialContext, _Mapping]] = ..., posture_analysis: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class RuntimeEventIds(_message.Message):
    __slots__ = ("conversation_id", "run_id", "tool_call_id", "interaction_id")
    CONVERSATION_ID_FIELD_NUMBER: _ClassVar[int]
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    TOOL_CALL_ID_FIELD_NUMBER: _ClassVar[int]
    INTERACTION_ID_FIELD_NUMBER: _ClassVar[int]
    conversation_id: str
    run_id: str
    tool_call_id: str
    interaction_id: str
    def __init__(self, conversation_id: _Optional[str] = ..., run_id: _Optional[str] = ..., tool_call_id: _Optional[str] = ..., interaction_id: _Optional[str] = ...) -> None: ...

class RuntimeEvent(_message.Message):
    __slots__ = ("version", "seq", "ids", "agent_configuration", "text_delta", "tool_call", "tool_result", "extracted_info", "lifestyle_context", "interaction_required", "phase_changed", "citation_added", "answer_attribution_added", "knowledge_gap", "red_flag_detected", "output_reviewed", "output_rejected", "usage_reported", "stream_done", "stream_error")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    SEQ_FIELD_NUMBER: _ClassVar[int]
    IDS_FIELD_NUMBER: _ClassVar[int]
    AGENT_CONFIGURATION_FIELD_NUMBER: _ClassVar[int]
    TEXT_DELTA_FIELD_NUMBER: _ClassVar[int]
    TOOL_CALL_FIELD_NUMBER: _ClassVar[int]
    TOOL_RESULT_FIELD_NUMBER: _ClassVar[int]
    EXTRACTED_INFO_FIELD_NUMBER: _ClassVar[int]
    LIFESTYLE_CONTEXT_FIELD_NUMBER: _ClassVar[int]
    INTERACTION_REQUIRED_FIELD_NUMBER: _ClassVar[int]
    PHASE_CHANGED_FIELD_NUMBER: _ClassVar[int]
    CITATION_ADDED_FIELD_NUMBER: _ClassVar[int]
    ANSWER_ATTRIBUTION_ADDED_FIELD_NUMBER: _ClassVar[int]
    KNOWLEDGE_GAP_FIELD_NUMBER: _ClassVar[int]
    RED_FLAG_DETECTED_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_REVIEWED_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_REJECTED_FIELD_NUMBER: _ClassVar[int]
    USAGE_REPORTED_FIELD_NUMBER: _ClassVar[int]
    STREAM_DONE_FIELD_NUMBER: _ClassVar[int]
    STREAM_ERROR_FIELD_NUMBER: _ClassVar[int]
    version: int
    seq: int
    ids: RuntimeEventIds
    agent_configuration: AgentConfigurationHandshake
    text_delta: MessageTextDelta
    tool_call: ToolCall
    tool_result: ToolResult
    extracted_info: ExtractedInfoUpsert
    lifestyle_context: LifestyleContextUpsert
    interaction_required: InteractionRequired
    phase_changed: PhaseChanged
    citation_added: CitationAdded
    answer_attribution_added: AnswerAttributionAdded
    knowledge_gap: KnowledgeGap
    red_flag_detected: RedFlagDetected
    output_reviewed: SafetyOutputEvent
    output_rejected: SafetyOutputEvent
    usage_reported: UsageReported
    stream_done: StreamDone
    stream_error: StreamError
    def __init__(self, version: _Optional[int] = ..., seq: _Optional[int] = ..., ids: _Optional[_Union[RuntimeEventIds, _Mapping]] = ..., agent_configuration: _Optional[_Union[AgentConfigurationHandshake, _Mapping]] = ..., text_delta: _Optional[_Union[MessageTextDelta, _Mapping]] = ..., tool_call: _Optional[_Union[ToolCall, _Mapping]] = ..., tool_result: _Optional[_Union[ToolResult, _Mapping]] = ..., extracted_info: _Optional[_Union[ExtractedInfoUpsert, _Mapping]] = ..., lifestyle_context: _Optional[_Union[LifestyleContextUpsert, _Mapping]] = ..., interaction_required: _Optional[_Union[InteractionRequired, _Mapping]] = ..., phase_changed: _Optional[_Union[PhaseChanged, _Mapping]] = ..., citation_added: _Optional[_Union[CitationAdded, _Mapping]] = ..., answer_attribution_added: _Optional[_Union[AnswerAttributionAdded, _Mapping]] = ..., knowledge_gap: _Optional[_Union[KnowledgeGap, _Mapping]] = ..., red_flag_detected: _Optional[_Union[RedFlagDetected, _Mapping]] = ..., output_reviewed: _Optional[_Union[SafetyOutputEvent, _Mapping]] = ..., output_rejected: _Optional[_Union[SafetyOutputEvent, _Mapping]] = ..., usage_reported: _Optional[_Union[UsageReported, _Mapping]] = ..., stream_done: _Optional[_Union[StreamDone, _Mapping]] = ..., stream_error: _Optional[_Union[StreamError, _Mapping]] = ...) -> None: ...

class AgentConfigurationHandshake(_message.Message):
    __slots__ = ("agent_configuration", "execution_provenance")
    AGENT_CONFIGURATION_FIELD_NUMBER: _ClassVar[int]
    EXECUTION_PROVENANCE_FIELD_NUMBER: _ClassVar[int]
    agent_configuration: AgentConfiguration
    execution_provenance: ExecutionProvenance
    def __init__(self, agent_configuration: _Optional[_Union[AgentConfiguration, _Mapping]] = ..., execution_provenance: _Optional[_Union[ExecutionProvenance, _Mapping]] = ...) -> None: ...

class AgentConfiguration(_message.Message):
    __slots__ = ("id", "role", "manifest_revision", "logical_model", "model_group_revision", "prompt_revision", "tool_policy_revision", "governance_policy_revision", "decision_policy_revision")
    ID_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    MANIFEST_REVISION_FIELD_NUMBER: _ClassVar[int]
    LOGICAL_MODEL_FIELD_NUMBER: _ClassVar[int]
    MODEL_GROUP_REVISION_FIELD_NUMBER: _ClassVar[int]
    PROMPT_REVISION_FIELD_NUMBER: _ClassVar[int]
    TOOL_POLICY_REVISION_FIELD_NUMBER: _ClassVar[int]
    GOVERNANCE_POLICY_REVISION_FIELD_NUMBER: _ClassVar[int]
    DECISION_POLICY_REVISION_FIELD_NUMBER: _ClassVar[int]
    id: str
    role: str
    manifest_revision: str
    logical_model: str
    model_group_revision: str
    prompt_revision: str
    tool_policy_revision: str
    governance_policy_revision: str
    decision_policy_revision: str
    def __init__(self, id: _Optional[str] = ..., role: _Optional[str] = ..., manifest_revision: _Optional[str] = ..., logical_model: _Optional[str] = ..., model_group_revision: _Optional[str] = ..., prompt_revision: _Optional[str] = ..., tool_policy_revision: _Optional[str] = ..., governance_policy_revision: _Optional[str] = ..., decision_policy_revision: _Optional[str] = ...) -> None: ...

class ExecutionProvenance(_message.Message):
    __slots__ = ("status", "runtime", "logical_model", "model_group_revision", "usage")
    STATUS_FIELD_NUMBER: _ClassVar[int]
    RUNTIME_FIELD_NUMBER: _ClassVar[int]
    LOGICAL_MODEL_FIELD_NUMBER: _ClassVar[int]
    MODEL_GROUP_REVISION_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    status: str
    runtime: str
    logical_model: str
    model_group_revision: str
    usage: _struct_pb2.Struct
    def __init__(self, status: _Optional[str] = ..., runtime: _Optional[str] = ..., logical_model: _Optional[str] = ..., model_group_revision: _Optional[str] = ..., usage: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class MessageTextDelta(_message.Message):
    __slots__ = ("delta",)
    DELTA_FIELD_NUMBER: _ClassVar[int]
    delta: str
    def __init__(self, delta: _Optional[str] = ...) -> None: ...

class ToolCall(_message.Message):
    __slots__ = ("tool", "args")
    TOOL_FIELD_NUMBER: _ClassVar[int]
    ARGS_FIELD_NUMBER: _ClassVar[int]
    tool: str
    args: _struct_pb2.Struct
    def __init__(self, tool: _Optional[str] = ..., args: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class ToolResult(_message.Message):
    __slots__ = ("tool", "result")
    TOOL_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    tool: str
    result: _struct_pb2.Struct
    def __init__(self, tool: _Optional[str] = ..., result: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class ExtractedInfoUpsert(_message.Message):
    __slots__ = ("info",)
    INFO_FIELD_NUMBER: _ClassVar[int]
    info: _struct_pb2.Struct
    def __init__(self, info: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class LifestyleContextUpsert(_message.Message):
    __slots__ = ("context",)
    CONTEXT_FIELD_NUMBER: _ClassVar[int]
    context: _struct_pb2.Struct
    def __init__(self, context: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class InteractionRequired(_message.Message):
    __slots__ = ("interaction_id", "question")
    INTERACTION_ID_FIELD_NUMBER: _ClassVar[int]
    QUESTION_FIELD_NUMBER: _ClassVar[int]
    interaction_id: str
    question: _struct_pb2.Struct
    def __init__(self, interaction_id: _Optional[str] = ..., question: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class PhaseChanged(_message.Message):
    __slots__ = ("to", "reason")
    FROM_FIELD_NUMBER: _ClassVar[int]
    TO_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    to: str
    reason: str
    def __init__(self, to: _Optional[str] = ..., reason: _Optional[str] = ..., **kwargs) -> None: ...

class CitationAdded(_message.Message):
    __slots__ = ("citation",)
    CITATION_FIELD_NUMBER: _ClassVar[int]
    citation: _struct_pb2.Struct
    def __init__(self, citation: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class AnswerAttributionAdded(_message.Message):
    __slots__ = ("attribution",)
    ATTRIBUTION_FIELD_NUMBER: _ClassVar[int]
    attribution: _struct_pb2.Struct
    def __init__(self, attribution: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class KnowledgeGap(_message.Message):
    __slots__ = ("query", "message")
    QUERY_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    query: str
    message: str
    def __init__(self, query: _Optional[str] = ..., message: _Optional[str] = ...) -> None: ...

class RedFlagDetected(_message.Message):
    __slots__ = ("has_red_flags", "flags")
    HAS_RED_FLAGS_FIELD_NUMBER: _ClassVar[int]
    FLAGS_FIELD_NUMBER: _ClassVar[int]
    has_red_flags: bool
    flags: _containers.RepeatedCompositeFieldContainer[_struct_pb2.Struct]
    def __init__(self, has_red_flags: _Optional[bool] = ..., flags: _Optional[_Iterable[_Union[_struct_pb2.Struct, _Mapping]]] = ...) -> None: ...

class SafetyOutputEvent(_message.Message):
    __slots__ = ("kind", "verdict", "reasons", "issues", "safety_fallback")
    KIND_FIELD_NUMBER: _ClassVar[int]
    VERDICT_FIELD_NUMBER: _ClassVar[int]
    REASONS_FIELD_NUMBER: _ClassVar[int]
    ISSUES_FIELD_NUMBER: _ClassVar[int]
    SAFETY_FALLBACK_FIELD_NUMBER: _ClassVar[int]
    kind: str
    verdict: str
    reasons: _containers.RepeatedScalarFieldContainer[str]
    issues: _struct_pb2.ListValue
    safety_fallback: str
    def __init__(self, kind: _Optional[str] = ..., verdict: _Optional[str] = ..., reasons: _Optional[_Iterable[str]] = ..., issues: _Optional[_Union[_struct_pb2.ListValue, _Mapping]] = ..., safety_fallback: _Optional[str] = ...) -> None: ...

class UsageReported(_message.Message):
    __slots__ = ("usage",)
    USAGE_FIELD_NUMBER: _ClassVar[int]
    usage: _struct_pb2.Struct
    def __init__(self, usage: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class StreamDone(_message.Message):
    __slots__ = ("response_id", "usage", "governance")
    RESPONSE_ID_FIELD_NUMBER: _ClassVar[int]
    USAGE_FIELD_NUMBER: _ClassVar[int]
    GOVERNANCE_FIELD_NUMBER: _ClassVar[int]
    response_id: str
    usage: _struct_pb2.Struct
    governance: _struct_pb2.Struct
    def __init__(self, response_id: _Optional[str] = ..., usage: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., governance: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class StreamError(_message.Message):
    __slots__ = ("message",)
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    message: str
    def __init__(self, message: _Optional[str] = ...) -> None: ...
