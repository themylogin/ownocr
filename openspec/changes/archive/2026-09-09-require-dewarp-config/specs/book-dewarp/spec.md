## REMOVED Requirements

### Requirement: Dewarp is optional configuration

**Reason**: The server now requires the object storage and endpoint settings to
start. A running server always holds them, so no request can find them missing.
The three scenarios describe states the system can no longer reach: the server
never starts without the settings, and the Dewarp action never reports them
absent for one book or for a selection.

**Migration**: Nothing replaces the requirement here. A server that refuses to
start on incomplete configuration is a deployment rule, not behavior that the
dewarp capability promises. The operator sets `AWS_ACCESS_KEY_ID`,
`AWS_SECRET_ACCESS_KEY`, `AWS_BUCKET`, `DEWARP_API_URL` and `DEWARP_API_TOKEN`
before the first start. The README variable table and `.env.example` mark all
five as required.
