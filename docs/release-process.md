# Release Process

The following steps are needed to create a release.

1. **Generate the Changelog**
   - Run the following command to generate the changelog on the `main` branch:

     ```bash
     make changelog
     ```

   - Create a new branch for the release:

     ```bash
     export VERSION=v0.0.1
     git checkout -b <user>/release-${VERSION}
     ```

2. **Review and Update the Changelog**
   - Review the generated changelog and make updates if necessary.
   - If needed (e.g., for a minor release), manually update the version in
   `.punch_version.py` to match the release version.
   - Add a `Deployment Notes` section to the changelog, if relevant or needed.

3. **Create and Merge a Pull Request**
   - Create a pull request from the `<user>/release-${VERSION}` branch.
   - Get the pull request reviewed and merged into `main`.

4. **Pull the Latest Changes**
   - Switch to the `main` branch and pull the latest changes:

     ```bash
     git checkout main
     git pull
     ```

5. **Create the Release Tag**
   - Run the following command to create the release tag:

     ```bash
     make release-tag
     ```

   - This will trigger the `release` GitHub Action to create and publish the
   release, including the changelog with changes since the previous release.
