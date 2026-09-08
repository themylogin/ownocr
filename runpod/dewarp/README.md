# Dewarp worker

A RunPod serverless worker. It reads page images from a bucket, straightens
them on a GPU, and writes the results back to the same bucket. PaddleX does the
work: `PP-LCNet_x1_0_doc_ori` finds the orientation and `UVDoc` unwarps the
page.

The image also keeps the command line tool it grew from.

## Build and push

Put a Hugging Face read token in `hf_token` first. The build downloads the two
models with it and bakes them into the image, so a running worker needs no
network for models.

```sh
IMAGE=docker.io/<user>/ownocr-dewarp:1 ./build.sh --push
```

The build needs a machine with Docker and BuildKit. It needs no GPU. Running
the worker does.

## Create the endpoint

1. Open RunPod, then Serverless, then New Endpoint.
2. Choose your image, for example `docker.io/<user>/ownocr-dewarp:1`.
3. Pick a GPU with 16 GB or more. Set the container disk to 20 GB.
4. Set active workers to 0 and max workers to 1. One book is one job.
5. Set the idle timeout to 60 seconds or more. A warm worker skips the model
   load.
6. Copy the endpoint id. The server wants the base URL:
   `https://api.runpod.ai/v2/<endpoint-id>`.
7. Copy an API key from Settings. That is `DEWARP_API_TOKEN`.

Put both in the server's `.env`, with the bucket variables:

```
AWS_ENDPOINT_URL=...
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=...
AWS_SECRET_ACCESS_KEY=...
AWS_BUCKET=ownocr
DEWARP_API_URL=https://api.runpod.ai/v2/<endpoint-id>
DEWARP_API_TOKEN=...
```

The endpoint reaches the bucket with these credentials. Use a key pair that
reaches that bucket and nothing else.

## Job contract

The server posts to `/run` and polls `/status/{id}`.

Input:

```json
{
  "input": {
    "bucket": "ownocr",
    "endpoint_url": "https://s3.example.com",
    "region": "us-east-1",
    "aws_access_key_id": "...",
    "aws_secret_access_key": "...",
    "input_prefix": "dewarp/<book key>/in/",
    "output_prefix": "dewarp/<book key>/out/",
    "jpeg_quality": 95
  }
}
```

`endpoint_url` is optional. Leave it out for real AWS.

Output:

```json
{
  "pages": [
    {
      "input_key": "dewarp/<book key>/in/0001.jpg",
      "output_key": "dewarp/<book key>/out/0001.jpg",
      "width": 2480,
      "height": 3508,
      "angle": 0
    }
  ],
  "failed": [
    { "input_key": "dewarp/<book key>/in/0002.jpg", "error": "could not decode" }
  ]
}
```

The output name is the input name with a `.jpg` suffix, so the caller pairs
them. The handler reads every object under `input_prefix` in name order. It
deletes nothing: the server owns the objects.

The worker loads the models once per process and reuses them. A page that
cannot be read lands in `failed`, and the other pages still finish.

## Command line

The image still runs the tool directly:

```sh
docker run --rm --gpus all -v "$PWD:/data" --entrypoint python \
    docker.io/<user>/ownocr-dewarp:1 /opt/docpp.py -i /data/input -o /data/output
```

`docpp.py --help` lists the options: device, batch size, PNG output, recursion,
and the two model switches.

## Files

| File                    | Purpose                                        |
| ----------------------- | ---------------------------------------------- |
| `Dockerfile`            | CUDA image, PaddleX, the models, the handler.   |
| `build.sh`              | Build and push with the token as a secret.      |
| `handler.py`            | The serverless handler. S3 in, S3 out.          |
| `docpp.py`              | The pipeline and the command line tool.         |
| `fetch_models.py`       | Downloads the models at build time.             |
| `doc_preprocessor.yaml` | PaddleX pipeline definition.                    |
| `example/`              | The reference worker this one was modelled on.  |
