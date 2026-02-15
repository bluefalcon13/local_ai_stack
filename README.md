# local_ai_stack
a template for creating a local AI stack using docker, and cloudflare's zero-trust


## 🛠️ The Identity Sandbox
Before running the bootstrap on your host, you can verify the User/UID logic inside a controlled environment.

```bash
cd test
docker build -t sudoubuntu:latest .
cp ../.env ./
cp ../setup_stack.sh ./
```

Edit the ./test/setup_stack.sh and comment out all lines below "# --- 3. Prepare Data Directories ---".
Then launch the container with the following docker run command:
```bash
docker run -it --rm -v ./:/workdir --workdir=/workdir sudoubuntu:latest bash
```
Inside the container you can run various scenarios, and edit the .env file to cause different cases.
This was not rigorously tested, but it was tested enough to make me happy to make these tools available to anyone else, assuming they are careful.
