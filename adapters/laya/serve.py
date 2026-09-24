"""Managed Laya adapter entrypoint.

The control plane downloads and verifies model files before invoking this file.
This process does not download weights from a model id; it loads only the local
directory passed by the control plane.
"""
import argparse
import os


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--model-dir", required=True)
    parser.add_argument("--variant", required=True)
    parser.add_argument("--device", default="")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, required=True)
    args = parser.parse_args()
    from laya import Agent
    from laya.serve import create_app

    class LocalAgent:
        """Adapter facade that makes every request use the verified local checkpoint."""

        def __init__(self):
            import torch

            if args.device == "cuda" and not torch.cuda.is_available():
                raise RuntimeError("CUDA profile requested but CUDA is unavailable in this PyTorch environment")
            if args.device == "mps" and not (hasattr(torch.backends, "mps") and torch.backends.mps.is_available()):
                raise RuntimeError("MPS profile requested but Apple MPS is unavailable")
            self.agent = Agent(args.model_dir, device=(args.device or None))
            if args.device and str(self.agent.device) != args.device:
                raise RuntimeError(f"requested device {args.device} but Laya loaded on {self.agent.device}")
            self.variant = args.variant
            os.environ["LAYA_DEVICE"] = str(self.agent.device)

        @property
        def loaded(self):
            return [self.variant]

        def predict(self, state, questions, model=None):
            result = self.agent.system_one(state, questions)
            result["routing"] = {"model": self.variant, "reason": "managed_local_variant"}
            return result

    # Agent receives a filesystem path, so Laya cannot invoke snapshot_download
    # or resolve an unpinned Hub revision during adapter startup or inference.
    router = LocalAgent()
    app = create_app(router=router)
    import uvicorn
    uvicorn.run(app, host=args.host, port=args.port, log_level="info")


if __name__ == "__main__":
    main()
