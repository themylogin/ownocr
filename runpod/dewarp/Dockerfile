# syntax=docker/dockerfile:1.7
FROM nvidia/cuda:12.6.3-cudnn-runtime-ubuntu24.04

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

RUN apt-get update && apt-get install -y --no-install-recommends \
        python3.12 python3.12-venv python3-pip \
        libgl1 libglib2.0-0 libgomp1 ccache \
    && rm -rf /var/lib/apt/lists/*

ENV VIRTUAL_ENV=/opt/venv
RUN python3.12 -m venv $VIRTUAL_ENV
ENV PATH="$VIRTUAL_ENV/bin:$PATH"

ARG PADDLE_VERSION=3.3.1
ARG PADDLEX_VERSION=3.7.2
RUN pip install --no-cache-dir -U pip setuptools wheel && \
    pip install --no-cache-dir "paddlepaddle-gpu==${PADDLE_VERSION}" \
        -i https://www.paddlepaddle.org.cn/packages/stable/cu126/ && \
    pip install --no-cache-dir "paddlex[ocr]==${PADDLEX_VERSION}" && \
    pip install --no-cache-dir opencv-python-headless

ENV PADDLE_PDX_CACHE_HOME=/opt/paddlex \
    HF_HOME=/opt/hf
COPY fetch_models.py /opt/fetch_models.py
RUN --mount=type=secret,id=hf_token,required=true \
    HF_TOKEN="$(cat /run/secrets/hf_token)" python /opt/fetch_models.py && \
    rm -rf /opt/hf

ENV HF_HUB_OFFLINE=1 \
    TRANSFORMERS_OFFLINE=1 \
    PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=True \
    PADDLE_PDX_MODEL_SOURCE=huggingface \
    GLOG_minloglevel=2 \
    FLAGS_call_stack_level=1 \
    HOME=/tmp \
    MPLCONFIGDIR=/tmp/mpl

ENV PADDLE_PDX_CACHE_HOME=/tmp/paddlex

ARG RUNPOD_VERSION=1.12.0
RUN pip install --no-cache-dir "runpod==${RUNPOD_VERSION}" boto3

COPY doc_preprocessor.yaml /opt/doc_preprocessor.yaml
COPY docpp.py /opt/docpp.py
COPY handler.py /opt/handler.py

# handler.py imports docpp, and the command line tool still runs from any
# working directory:
#   docker run --rm --gpus all -v "$PWD:/data" --entrypoint python IMAGE \
#       /opt/docpp.py -i /data/input -o /data/output
ENV PYTHONPATH=/opt

WORKDIR /data
ENTRYPOINT []
CMD ["python", "-u", "/opt/handler.py"]
