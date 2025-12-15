# Plugin release process

Follow these steps to release a new version of the plugins:
  1. Create a tag with the release version number in the format vX.Y.Z (e.g., v0.2.0 creates a release version 0.2.0).
  2. Push the tag to the repository.
  3. GitHub Actions will automatically create a draft release for the release tag.
  4. Once the release workflow completes, update the release notes, uncheck "Draft", and publish the release.

Note: Release tags (of the form v*) are protected and can only be pushed by organization and/or repository owners.