"""Run the real Diagnosis application path against the qualification dataset."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from pydantic_ai import ToolCallPart, capture_run_messages
from pydantic_ai.models.test import TestModel

from src.configuration.diagnosis_agent_config import DiagnosisAgentManifest
from src.evals.agent_config_archive import get_diagnosis_evaluation_configuration
from src.evals.diagnosis_qualification_dataset import (
    dataset_fingerprint,
    load_dataset_document,
    load_diagnosis_dataset,
)
from src.evals.diagnosis_qualification_schema import (
    DEFAULT_DATASET_PATH,
    DEFAULT_QUALIFICATION_CONFIGURATION_ID,
    DiagnosisEvalExecution,
    DiagnosisEvalInputs,
    DiagnosisEvalTrace,
    DiagnosisQualificationRun,
)
from src.evals.retired_diagnosis_runtime import (
    RETIRED_DIAGNOSIS_TOOL_POLICY_V1,
    OfflineHistoricalDiagnosisService,
)
from src.services.diagnosis_service import DiagnosisService
from src.testing_support.deterministic_ai import deterministic_diagnosis_model


def _configured_deterministic_service(
    config: DiagnosisAgentManifest,
    deterministic_output: dict[str, Any] | None = None,
) -> tuple[DiagnosisService, Any]:
    model = (
        TestModel(call_tools=[], custom_output_args=deterministic_output)
        if deterministic_output is not None
        else deterministic_diagnosis_model(call_tools=[])
    )

    def resolve_configuration(configuration_id: str) -> DiagnosisAgentManifest:
        if configuration_id != config.configuration_id:
            raise ValueError(
                f"qualification attempted unexpected Diagnosis configuration_id: {configuration_id}"
            )
        return config

    service_type = (
        OfflineHistoricalDiagnosisService
        if config.tool_policy_revision == RETIRED_DIAGNOSIS_TOOL_POLICY_V1
        else DiagnosisService
    )
    return service_type(
        model_resolver=lambda _config: model,
        configuration_resolver=resolve_configuration,
    ), model


def build_deterministic_task(configuration_id: str | None = None) -> Any:
    """Instrument the real Diagnosis application path without provider nondeterminism."""

    config = (
        get_diagnosis_evaluation_configuration(configuration_id)
        if configuration_id is not None
        else get_diagnosis_evaluation_configuration(DEFAULT_QUALIFICATION_CONFIGURATION_ID)
    )

    async def task(inputs: DiagnosisEvalInputs) -> DiagnosisEvalExecution:
        service, model = _configured_deterministic_service(config, inputs.model_output)
        with capture_run_messages() as messages:
            payload = await service.generate_diagnosis(
                user_id=inputs.user_id,
                body_state_revision=inputs.body_state_revision,
                configuration_id=config.configuration_id,
                body_state=inputs.body_state,
                relevant_history=inputs.relevant_history,
                profile=inputs.profile,
                safety_envelope=inputs.safety_envelope,
            )

        parameters = getattr(model, "last_model_request_parameters", None)
        available_tools = (
            [tool.name for tool in parameters.function_tools] if parameters is not None else []
        )
        tool_calls = [
            part.tool_name
            for message in messages
            for part in message.parts
            if isinstance(part, ToolCallPart) and part.tool_name != "final_result"
        ]
        return DiagnosisEvalExecution(
            payload=payload,
            configuration_id=config.configuration_id,
            trace=DiagnosisEvalTrace(
                agent_executed=parameters is not None,
                available_tools=available_tools,
                tool_calls=tool_calls,
            ),
        )

    return task


def run_diagnosis_qualification(
    path: Path = DEFAULT_DATASET_PATH,
    *,
    configuration_id: str | None = None,
) -> DiagnosisQualificationRun:
    """Evaluate one immutable Agent configuration against the qualification dataset."""

    config = (
        get_diagnosis_evaluation_configuration(configuration_id)
        if configuration_id is not None
        else get_diagnosis_evaluation_configuration(DEFAULT_QUALIFICATION_CONFIGURATION_ID)
    )
    document = load_dataset_document(path)
    dataset = load_diagnosis_dataset(path, configuration_id=config.configuration_id)
    report = dataset.evaluate_sync(
        build_deterministic_task(config.configuration_id),
        progress=False,
        name="diagnosis-qualification",
    )
    return DiagnosisQualificationRun(
        report=report,
        dataset_path=path,
        dataset_fingerprint=dataset_fingerprint(document),
        configuration=config,
    )
