# Releasing Sigryx

This is the maintainer cheat sheet for publishing a Sigryx release.

## 1. Choose the version

Use semantic versioning:

- patch: `v1.0.1 -> v1.0.2` for fixes;
- minor: `v1.0.2 -> v1.1.0` for backward-compatible features;
- major: `v1.x.x -> v2.0.0` for breaking changes.

Set the version once for the shell session:

```bash
export VERSION=v1.0.2
export IMAGE=rajabinekoo/sigryx
```

Do not release `latest` without also publishing an immutable version tag.

## 2. Start from the release commit

Make sure the working tree is clean and the branch is up to date:

```bash
git status
git pull --ff-only
```

Run the normal checks:

```bash
make fmt
make vet
make test
make test-race
make docs-build
```

Then smoke-test the Docker stack from a fresh database:

```bash
docker compose down -v
docker compose up --build -d
docker compose ps
docker compose logs --tail=100 sigryx
curl --fail http://localhost:8080/v1/health
```

Before continuing, verify that Sigryx is healthy and Atlas migrations completed without errors.

## 3. Build the release image locally

Build the exact release candidate first:

```bash
docker build -t ${IMAGE}:${VERSION} .
```

Inspect the bundled Atlas binary if needed:

```bash
docker run --rm ${IMAGE}:${VERSION} atlas version
```

Do not push until this local image is known to start correctly.

## 4. Tag the release commit

Create an annotated Git tag from the exact commit being released:

```bash
git tag -a ${VERSION} -m "Sigryx ${VERSION}"
git push origin ${VERSION}
```

The Git tag, Docker version tag, and GitHub Release must use the same version.

## 5. Publish the Docker image

Log in once if required:

```bash
docker login
```

Publish multi-architecture images for AMD64 and ARM64:

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ${IMAGE}:${VERSION} \
  -t ${IMAGE}:latest \
  --push \
  .
```

Production deployments should pin `${IMAGE}:${VERSION}`. `latest` is only a convenience pointer to the newest stable release.

## 6. Verify the published image

Pull the image back from Docker Hub instead of trusting the local build cache:

```bash
docker pull ${IMAGE}:${VERSION}
docker buildx imagetools inspect ${IMAGE}:${VERSION}
```

Then test the published image in a real Compose setup or integration environment. Confirm at minimum:

```text
/v1/health
/docs
/openapi.json
```

Also verify that startup migrations work against a fresh PostgreSQL database and against the intended upgrade path when the release contains database changes.

## 7. Publish the GitHub Release

Create a GitHub Release for the existing `${VERSION}` tag.

Use:

```text
Title: Sigryx vX.Y.Z
Tag:   vX.Y.Z
```

Keep the release notes short and useful:

- important features;
- bug fixes;
- security-relevant changes;
- migration or deployment notes;
- breaking changes, if any.

## Release checklist

```text
[ ] VERSION is correct
[ ] working tree is clean
[ ] tests pass
[ ] docs build passes
[ ] fresh Docker Compose smoke test passes
[ ] migrations complete successfully
[ ] local release image starts successfully
[ ] annotated Git tag is pushed
[ ] versioned Docker image is pushed
[ ] latest points to the same stable release
[ ] published Docker image is pulled and verified
[ ] GitHub Release is published
```

## The short version

```bash
export VERSION=v1.0.2
export IMAGE=rajabinekoo/sigryx

make fmt
make vet
make test
make test-race
make docs-build

docker compose down -v
docker compose up --build -d
curl --fail http://localhost:8080/v1/health

git tag -a ${VERSION} -m "Sigryx ${VERSION}"
git push origin ${VERSION}

docker login
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ${IMAGE}:${VERSION} \
  -t ${IMAGE}:latest \
  --push \
  .

docker pull ${IMAGE}:${VERSION}
docker buildx imagetools inspect ${IMAGE}:${VERSION}
```

After that, publish the GitHub Release for the same tag.
