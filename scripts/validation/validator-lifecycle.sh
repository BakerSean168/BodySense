#!/usr/bin/env bash

validator_create_builder() {
  local builder_name="$1"
  docker buildx create \
    --name "$builder_name" \
    --driver docker-container \
    >/dev/null
}

validator_project_image_ids() {
  local project_name="$1"
  docker image ls \
    --filter "label=com.docker.compose.project=${project_name}" \
    --format '{{.ID}}' \
    | awk 'NF' \
    | sort -u
}

validator_builder_exists() {
  local builder_name="$1"
  local builders=""
  builders="$(docker buildx ls --format '{{.Name}}' 2>/dev/null)" || return 2
  grep -Fxq "$builder_name" <<<"$builders"
}

validator_remove_builder() {
  local builder_name="$1"
  local max_attempts="${VALIDATOR_BUILDER_REMOVE_ATTEMPTS:-3}"
  local retry_delay="${VALIDATOR_BUILDER_REMOVE_RETRY_SECONDS:-1}"
  local attempt=1
  local exists_rc=0

  while (( attempt <= max_attempts )); do
    if docker buildx rm -f "$builder_name" >/dev/null 2>&1; then
      return 0
    fi

    exists_rc=0
    validator_builder_exists "$builder_name" || exists_rc="$?"
    if [[ "$exists_rc" -eq 1 ]]; then
      echo "VALIDATOR_TEARDOWN_STEP=RECOVERED step=builder-remove builder=${builder_name} attempt=${attempt}"
      return 0
    fi
    if [[ "$exists_rc" -eq 2 ]]; then
      return 2
    fi

    if (( attempt < max_attempts )); then
      sleep "$retry_delay"
    fi
    attempt=$((attempt + 1))
  done

  return 1
}

validator_cleanup() {
  local repo_root="$1"
  local project_name="$2"
  local builder_name="$3"
  local builder_created="${4:-0}"
  local keep_stack="${5:-0}"
  local failed=0
  local image_output=""
  local -a image_ids=()
  local -a compose=(
    docker compose
    -f "$repo_root/docker/docker-compose.yml"
    --profile dev
    -p "$project_name"
  )

  if [[ "$keep_stack" == "1" ]]; then
    echo "VALIDATOR_TEARDOWN=SKIP reason=keep-validator-stack project=${project_name} builder=${builder_name}"
    return 0
  fi

  if ! "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1; then
    echo "VALIDATOR_TEARDOWN_STEP=FAIL step=compose-down project=${project_name}" >&2
    failed=1
  fi

  if image_output="$(validator_project_image_ids "$project_name" 2>/dev/null)"; then
    if [[ -n "$image_output" ]]; then
      mapfile -t image_ids <<<"$image_output"
      if ! docker image rm -f "${image_ids[@]}" >/dev/null 2>&1; then
        echo "VALIDATOR_TEARDOWN_STEP=FAIL step=image-remove project=${project_name}" >&2
        failed=1
      fi
    fi
  else
    echo "VALIDATOR_TEARDOWN_STEP=FAIL step=image-discovery project=${project_name}" >&2
    failed=1
  fi

  if [[ "$builder_created" == "1" ]]; then
    if ! validator_remove_builder "$builder_name"; then
      echo "VALIDATOR_TEARDOWN_STEP=FAIL step=builder-remove builder=${builder_name}" >&2
      failed=1
    fi
  fi

  if [[ "$failed" -ne 0 ]]; then
    echo "VALIDATOR_TEARDOWN=FAIL project=${project_name} builder=${builder_name}" >&2
    return 1
  fi

  echo "VALIDATOR_TEARDOWN=PASS project=${project_name} builder=${builder_name} images=${#image_ids[@]}"
}
