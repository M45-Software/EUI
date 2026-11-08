# goThoom integration

EUI's reusable controls, editors, layout, themes, and render pool live in this
module. Application concerns such as saving files, language tokenization, chat
history, completion providers, and game assets belong to the client.

## Check local changes against the client

From EUI, with Go 1.27.1 and Python 3 installed:

```sh
xvfb-run -a ./build-scripts/check-gothoom.sh /path/to/goThoom
```

The script copies the client's `source` directory and required test fixtures to a temporary location,
removes its embedded EUI package, rewrites imports to this module, and uses a
local Go module replacement. It resolves dependencies, runs the client's tests,
and builds the client with Cgo disabled. The original client checkout is not
modified. The client must already contain its ordinary build resources.

## Pin the shared implementation

After publishing the tested EUI revision:

1. Replace `gothoom/eui` imports, including `renderpool` and tests, with
   `github.com/Distortions81/EUI`.
2. Add the published EUI revision to the client's `go.mod`, run `go mod tidy`,
   and remove the client's embedded `source/eui` directory.
3. Replace the extraction check with tests against the standalone module, and
   update client development documentation to point here for shared UI fixes.
4. Run client tests and the normal desktop/release build checks. Configure
   authenticated module access on build hosts while EUI remains private.

Use a local replacement while developing both checkouts together. Release builds
must resolve a published, pinned revision so they do not depend on a sibling
checkout. Update the pinned dependency when adopting further toolkit fixes.

The canonical style name is `Default`. Theme recommendations and style selectors
use this name.
