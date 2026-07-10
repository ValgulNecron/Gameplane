# Bake definition for the e2e images (used by .github/actions/e2e-images).
# `docker buildx bake --load e2e` builds all three concurrently — the
# compile steps are independent, so this roughly halves image-build wall
# time versus three sequential `docker build`s. Tags match what
# `make e2e-images` produces and what deploy/kind/e2e.sh loads.
group "e2e" {
  targets = ["e2e-operator", "e2e-api", "e2e-agent"]
}

target "e2e-operator" {
  context    = "."
  dockerfile = "operator/Dockerfile"
  tags       = ["gameplane-test/operator:e2e"]
}

target "e2e-api" {
  context    = "."
  dockerfile = "api/Dockerfile"
  tags       = ["gameplane-test/api:e2e"]
}

target "e2e-agent" {
  context    = "."
  dockerfile = "agent/Dockerfile"
  tags       = ["gameplane-test/agent:e2e"]
}

# The headless protocol bot the game-bot job runs inside the cluster. It is
# deliberately outside the "e2e" group: only that one job needs it, and the
# other e2e buckets shouldn't pay to build it.
target "e2e-gameprobe" {
  context    = "."
  dockerfile = "test/e2e/Dockerfile"
  tags       = ["gameplane-test/gameprobe:e2e"]
}
