# Dewarp worker

A RunPod serverless worker. It reads page images from a bucket, straightens them
on a GPU, and writes the results back. PaddleX does the work:
`PP-LCNet_x1_0_doc_ori` finds the orientation and `UVDoc` unwarps the page.

## Build and push

Put a Hugging Face read token in `hf_token` first. The build downloads the two
models with it and bakes them into the image, so a worker needs no network for
models.

```sh
IMAGE=docker.io/<user>/ownocr-dewarp:1 ./build.sh --push
```

## Deploy on RunPod

1. Open RunPod, then Serverless, then New Endpoint.
2. Choose your image, for example `docker.io/<user>/ownocr-dewarp:1`.
3. Pick the GPU. Use an RTX 4090, or another GPU with compute capability 90 or
   lower. These all work: A4000, A4500, RTX 2000 Ada, RTX 4000 Ada, A5000, A40,
   L4, L40S, A100, H100.

   **Deselect every Blackwell GPU**, such as the RTX PRO 6000 and the B200, and
   the MIG slices of them. The image carries `paddlepaddle-gpu` for CUDA 12.6,
   which holds no kernels for those. The worker does not fail on one. Every page
   comes back blank instead. An endpoint takes any GPU you leave selected, so
   one Blackwell entry is enough to ruin a book.
4. Set the container disk to 20 GB.
5. Set active workers to 0 and max workers to 1. One book is one job.
6. Set the idle timeout to 60 seconds or more. A warm worker skips the model
   load, which takes about a minute.
7. Copy the endpoint id. The server wants the base URL:
   `https://api.runpod.ai/v2/<endpoint-id>`.
8. Copy an API key from Settings. That is `DEWARP_API_TOKEN`.

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

The worker loads the models once per process and reuses them. A page that cannot
be read lands in `failed`, and the other pages still finish.
