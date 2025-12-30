# Code-Server Controller

🚀 Dynamically deploy VS Code instances in your Kubernetes cluster via REST API

Create isolated code-server (VS Code in browser) instances on-demand through simple HTTP requests. Each user gets their own namespace with 10GB storage.


## Deploy

```bash
# Apply RBAC and controller
kubectl create ns code-server
kubectl apply -f https://raw.githubusercontent.com/mehdi-chebbi/server-code/mehdi-nightly/rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/mehdi-chebbi/server-code/mehdi-nightly/full-depl.yaml

# Verify it's running
kubectl get pods -n code-server

# Port-forward to access the API
kubectl port-forward -n code-server svc/code-server-controller 8081:8081
```

## API Endpoints

### Health Check
```bash
curl http://localhost:8081/health
```
**Response:**
```json
{
  "status": "healthy",
  "helm_available": true,
  "kubectl_available": true,
  "repo_cloned": true
}
```

---

### Create Code-Server Instance
```bash
curl -X POST http://localhost:8081/create \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "john",
    "password": "MyPassword123"
  }'
```

**Response:**
```json
{
  "message": "Code server deployed successfully via Helm",
  "release_name": "code-server-john",
  "namespace": "code-server-john",
  "password": "MyPassword123",
  "port_forward_command": "kubectl port-forward --namespace code-server-john service/code-server-john 8080:http",
  "access_url": "http://127.0.0.1:8080"
}
```

**What you get:**
- Separate namespace: `code-server-john`
- 10GB persistent storage
- Isolated VS Code instance
- Takes ~30-60 seconds to deploy

---

### List All Instances
```bash
curl http://localhost:8081/list
```

**Response:**
```json
{
  "releases": [
    {
      "release_name": "code-server-john",
      "namespace": "code-server-john",
      "status": "deployed",
      "chart": "code-server-4.104.2"
    }
  ],
  "count": 1
}
```

---

### Delete Instance
```bash
curl -X DELETE http://localhost:8081/delete \
  -H "Content-Type: application/json" \
  -d '{"user_id": "john"}'
```

**Response:**
```json
{
  "message": "Code server uninstalled successfully",
  "user_id": "john"
}
```

Deletes everything: namespace, pod, service, and storage.

---

## Access Your Code-Server

After creating an instance:

```bash
# Port-forward to your code-server
kubectl port-forward --namespace code-server-john service/code-server-john 8080:http

# Open http://localhost:8080 in your browser
# Enter the password from the create response
```

## Postman Usage

**Create Instance:**
- Method: `POST`
- URL: `http://localhost:8081/create`
- Headers: `Content-Type: application/json`
- Body (raw JSON):
```json
{
  "user_id": "john",
  "password": "MyPassword123"
}
```

**Delete Instance:**
- Method: `DELETE`
- URL: `http://localhost:8081/delete`
- Headers: `Content-Type: application/json`
- Body (raw JSON):
```json
{
  "user_id": "john"
}
```
