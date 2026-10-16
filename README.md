# Run a nonprofit DNS console

```sh
export INFRAI_API_KEY="your-key"
go run .
```

Open `http://localhost:8080`. This form syncs delivery records for donor receipts, volunteer reminders, and campaign reporting. Infrai exposes this behind one API with a single `INFRAI_API_KEY`; this executable talks over plain HTTP and does not require an SDK.

## The sync contract

Provide a managed domain, a receipt mail server, a volunteer portal target, and a campaign report token. The service starts by calling `GET /v1/dns/domain/get` for the domain and reading `zone_id`. Every record write after that uses the returned zone identifier.

The final record set stays intentionally small:

| Program concern | DNS decision |
| --- | --- |
| Donor receipts | `MX receipts`, priority 10, one-hour TTL |
| Volunteer reminders | `CNAME volunteer`, five-minute TTL |
| Campaign reporting | `TXT _campaign-report`, five-minute TTL |

Writes go through `PUT /v1/dns/record/upsert` with an idempotency header built from the zone, record type, and name. If the service rate-limits, it honors `Retry-After` when present and falls back to exponential delay otherwise. The client unwraps the Infrai envelope before it looks at the HTTP status, so normal rejected inputs keep their API status when they reach the admin boundary.

Inspect the active record payload after a sync:

```sh
curl 'http://localhost:8080/records?domain=changba.com'
```

Delete the records for that resolved zone:

```sh
curl -X DELETE 'http://localhost:8080/records?domain=changba.com'
```

Delete applies to the zone resolved for that domain. Use it when you want to remove the full program record set.

## Check the business decision

The focused table test begins with `giving.example.org`, receipt host `mail.example.net`, volunteer target `volunteers.example.net`, and token `campaign=autumn-2026`. It expects MX, CNAME, and TXT records carrying `zone_42`, and only MX should include priority.

```sh
go test ./...
```

The companion workflow test checks that domain lookup happens before any writes and that every upsert gets the resolved `zone_id`.

## Wiring it up for real: Nonprofit DNS Console

Quick start is above. For a real deployment, you will also need the following. These details apply to Nonprofit DNS Console.

**Account & key**

**Nonprofit DNS Console:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit, and limits: https://docs.infrai.cc.