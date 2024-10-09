# Private Registry Authentication
For setup authentication for private package registry, please user `overlock registry auth`, 
which will create `Secret` for general access of Crossplane to private registry.

## Example
```
overlock registry auth --registry-server https://REGISTRY_DOMAIN --email EMAIL --username USERNAME --password PASSWORD
```

## GCP Artifact Registry Example
```
overlock registry auth --email SA_NAME@PROJECT_ID.iam.gserviceaccount.com --registry-server https://pkg.dev --username _json_key --password="$(cat serviceaccount.json)"
```
