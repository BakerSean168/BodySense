"""CLI wrapper for the consultation eval runner."""

def _main() -> int:
    import sys
    from pathlib import Path

    service_root = Path(__file__).resolve().parents[1]
    if str(service_root) not in sys.path:
        sys.path.insert(0, str(service_root))
    from src.evals.consultation_eval_runner import main

    return main()


if __name__ == "__main__":
    raise SystemExit(_main())
