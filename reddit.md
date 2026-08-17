**Project Name:** Gameplane

**Repo/Website:** [Github](https://github.com/ValgulNecron/Gameplane) | [website](https://valgulnecron.github.io/gameplane-website/)

**Description:** I needed a single dashboard to easily manage my game server. at first I was using docker and the most logical choice for me at the time was AMP (CubeCoder) the issue is that this can't run inside a docker container and want to be installed on the host. Now that i have switched to kubernetes I had multiple choice having both kubernetes and docker on the same machine for game server (which seemed like a bad idea), run AMP on the host without docker isolation that is even worth a single compromised server is an host compromised, running vm (using kvm / kubevirt) for AMP what I was doing at the start, or switch to full kubernetes native but no GUI specificaly for game exist and a lot of my friend won't manage using direct kubectl or even lens. So I had the idea of creating a full dashboard for managing this.
This is still in beta, postgres db is experimental.  

**AI Involvement:** This was a project only done by AI, no code was written by human. every code change was checked by me and while i'm not a web dev and i'm also not really a go dev nothing seemed out of place or absolutly wrong or bad. for the how the ai was guided on what it needed everything touching web need to go trought a design phase first using pencil and the pencil mcp. once validated the ai write the code and open a pr I then check all pr once the ai is done and merge all once validated. 

**Deployment:** 

System requirement:
  - Kubernetes 1.28+
  - Helm 3.13+
  - default RWO StorageClass
 
Tested only on x86-64 cpu, does not own any arm (except my phone) or 32bits cpu
an arm image is also built but not tested.

[Install docs on github](https://github.com/ValgulNecron/Gameplane#install-on-a-cluster) or 
```sh
helm upgrade --install gameplane oci://ghcr.io/valgulnecron/charts/gameplane \
  --version 0.2.0-beta.7 \
  --namespace gameplane-system --create-namespace \
  --set ingress.host=gameplane.your-domain.test
  # --set ingress.tls=false # if you do not want tls, it require a cert manager annotation or a pre created tls cert

kubectl -n gameplane-system exec deploy/gameplane-api -- /api bootstrap-admin --username admin --password "<choose>"
# or
kubectl -n gameplane-system exec -i deploy/gameplane-api -- /api bootstrap-admin --username admin --password-stdin
```