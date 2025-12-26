## Usage

```bash
# Start everything
docker-compose up --build

# In another terminal, test proxy (should be denied)
curl http://localhost:8000

# Open admin interface
open http://localhost:8080

# Allow your IP (172.17.0.1 or similar), then retry
curl http://localhost:8000
```
