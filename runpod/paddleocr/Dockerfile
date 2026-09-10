# syntax=docker/dockerfile:1.7
# Base already carries vLLM + the PaddleOCR genai server (torch stack). The sm120
# tag targets Blackwell (SM 12.0); the paddle client wheel must match, hence cu129.
# We add a SEPARATE venv with paddle for the layout/orientation/unwarp models and
# the RunPod/S3 glue. The vLLM server runs as a subprocess, the paddle client in
# the main process, so the two CUDA frameworks never share an interpreter.
FROM ccr-2vdh3abv-pub.cnc.bj.baidubce.com/paddlepaddle/paddleocr-genai-vllm-server:latest-nvidia-gpu-sm120

USER root
ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

RUN apt-get update && apt-get install -y --no-install-recommends \
        libgl1 libglib2.0-0 libgomp1 \
    && rm -rf /var/lib/apt/lists/*

# --- client venv: paddle pipeline + RunPod/S3, isolated from the base torch env ---
# Use the base image's own python (Debian 12, /usr/local/bin/python 3.10).
ENV CLIENT_VENV=/opt/venv
RUN /usr/local/bin/python -m venv $CLIENT_VENV
ARG PADDLE_VERSION=3.2.2
ARG PADDLE_CUDA=cu129
ARG PADDLEX_VERSION=3.7.2
ARG RUNPOD_VERSION=1.12.0
RUN $CLIENT_VENV/bin/pip install --no-cache-dir -U pip setuptools wheel && \
    $CLIENT_VENV/bin/pip install --no-cache-dir "paddlepaddle-gpu==${PADDLE_VERSION}" \
        -i https://www.paddlepaddle.org.cn/packages/stable/${PADDLE_CUDA}/ && \
    $CLIENT_VENV/bin/pip install --no-cache-dir "paddlex[ocr]==${PADDLEX_VERSION}" \
        "paddleocr>=3.7" "openai>=1.63" opencv-python-headless \
        "runpod==${RUNPOD_VERSION}" boto3 huggingface_hub

# --- bake all four pipeline models into the image (offline at runtime) ---
ENV PADDLE_PDX_CACHE_HOME=/opt/paddlex \
    HF_HOME=/opt/hf
COPY fetch_models.py /opt/fetch_models.py
RUN --mount=type=secret,id=hf_token,required=true \
    HF_TOKEN="$(cat /run/secrets/hf_token)" $CLIENT_VENV/bin/python /opt/fetch_models.py && \
    rm -rf /opt/hf && chmod -R a+rwX /opt/paddlex

# runtime env: offline, quiet, writable HOME; both server and client read /opt/paddlex
ENV HF_HUB_OFFLINE=1 \
    TRANSFORMERS_OFFLINE=1 \
    PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True \
    PADDLE_PDX_MODEL_SOURCE=huggingface \
    PADDLE_PDX_CACHE_HOME=/opt/paddlex \
    GLOG_minloglevel=2 \
    FLAGS_call_stack_level=1 \
    HOME=/tmp \
    MPLCONFIGDIR=/tmp/mpl \
    PADDLEOCR_BIN=/usr/local/bin/paddleocr \
    VL_DEVICE=gpu:0 \
    VL_SERVER_PORT=8080

COPY vlocr.py /opt/vlocr.py
COPY ocr_v6.py /opt/ocr_v6.py
COPY handler.py /opt/handler.py
ENV PYTHONPATH=/opt

# Local (non-RunPod) test, e.g.:
#   docker run --rm --gpus all -v "$PWD:/data" --entrypoint /opt/venv/bin/python IMAGE \
#       /opt/vlocr.py -i /data/1.png -o /data/out
WORKDIR /data
ENTRYPOINT []
CMD ["/opt/venv/bin/python", "-u", "/opt/handler.py"]
