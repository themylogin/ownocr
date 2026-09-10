## Job contract

Input:

```json
{
  "input": {
    "bucket": "ownocr",
    "endpoint_url": "https://s3.example.com",
    "region": "us-east-1",
    "aws_access_key_id": "...",
    "aws_secret_access_key": "...",
    "input_prefix": "vl/<book key>/in/",
    "output_prefix": "vl/<book key>/out/",
    "modes": ["v6", "vl"]
  }
}
```

`endpoint_url` is optional (leave out for real AWS).

Output:

```json
{
  "pages": [
    { "input_key": "vl/<book>/in/0001.jpg",
      "output_keys": ["vl/<book>/out/0001.v6.json", "vl/<book>/out/0001.v6.txt"],
      "chars": 4041, "lines": 72, "seconds": 2.8 },
    { "input_key": "vl/<book>/in/0001.jpg",
      "output_keys": ["vl/<book>/out/0001.md", "vl/<book>/out/0001.json"],
      "chars": 4052, "blocks": 11, "seconds": 16.0 }
  ],
  "failed": [
    { "input_key": "vl/<book>/in/0002.jpg", "error": "..." }
  ]
}
```

## Modes

`modes` is **required** in the job input and is an **array** of the engines to run,
so a job can ask for more than one and new engines can be added later:

| `modes` | Engines | Outputs | Cold start |
|---|---|---|---|
| `["v6"]` | PP-OCRv6 only | `.v6.json`, `.v6.txt` | **~14 s** |
| `["vl"]` | PaddleOCR-VL | `.md`, `.json` | ~120 s |
| `["v6","vl"]` | both, run independently | all four | ~120 s |
