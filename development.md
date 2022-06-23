## Development

To start run
```
docker-compose up -d
```
This will start 2 containers:
- `github-pr-resource_source` - used to **build** resource bins; mapped to folder with the source code on host (execute build here)
- `github-pr-resource_exec` - used to **run** resource bins; `/opt/resource` folder with built bin files is mapped to build folder in `github-pr-resource_source` (execute bin command here)

### Build
1. Enter `source` container
```
docker exec -it github-pr-resource_source bash
```
2. Build from source
```
task build_dev
```

### Execute
1. Enter `exec` container
```
docker exec -it github-pr-resource_exec sh
```
2. Execute command, e.g.:
```
./check
```
and then paste input json with config
example
```
{
    "source": {
        "repository": "<<repository_name>>",
        "access_token": "<<place_your_github_pat_here>>",
        "status_context": "concourse-ci/status",
        "states": ["OPEN","MERGED"]
    } 
}
```

to add optional parameters to your query:
```
{
    "source": {
        "repository": "<<repository_name>>",
        "access_token": "<<place_your_github_pat_here>>",
        "status_context": "concourse-ci/status"
    },
    "page": {
        "sort_field": "UPDATED_AT",
        "sort_direction": "ASC",
        "max_prs": 600,
        "page_size": 25,
        "delay_between_pages": 1000,
        "max_retries": 3
    }
}
```

## Publishing resource image to Docker Hub


1. Sign in to Docker Hub account
2. Build image
```
docker build -t <<account_name>>/github-pr-resource:<<tagname>> .
```
3. Push image
```
docker push <<account_name>>/github-pr-resource:<<tagname>>
```
