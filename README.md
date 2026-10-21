# Circle

Circle is a native mobile app for people over 16 and the supervisors they choose. A person shares their precise location with that circle. A supervisor can start a safety check. If the person does not confirm they are safe, the supervisor sees the last 24 hours of their path, the places they saved, and a prompt to contact emergency services.

A self-harm concern also shows **988**, the US Suicide & Crisis Lifeline. Circle does not give crisis-intervention steps and does not replace emergency services.

There is no volunteer role and no nearby dispatch. A supervisor can see someone only after that person invites them.

## Age check

Signup asks for a passport, national ID, or driver's license: the name printed on it, the date of birth, the document number, and a photo. The account stays closed unless that date of birth is more than 16 years ago. The photo is stored on the API machine and is never sent to supervisors.

This build does not send the document to a government registry or a paid identity vendor. The screen says the age was taken from the submitted ID.

## Location

Sharing is off until the person turns it on. The app then asks for precise location, and for background updates so a trail still records while the phone is locked. Background updates need a development build. Expo Go, and the web preview, only send updates while Circle is open. A last point older than 15 minutes is labeled stale. The map draws an accuracy circle.

Redis holds the current point (`GEOADD geo:live`) and pushes it to supervisors over a WebSocket. Accounts, ID records, invites, places, safety checks, and the 24 hour trail stay in Postgres. If Redis is down, points still save and the app refreshes instead of moving the pin live.

## Run locally

Postgres and Redis:

```bash
docker compose up -d
docker compose exec postgres psql -U circle -d circle -c "CREATE DATABASE circle_test OWNER circle;"
```

`circle_test` is only for `go test`. Skip that command if the database already exists.

Without Docker, install PostgreSQL 16 and Redis, then create a role and the two databases:

```bash
sudo -u postgres psql -c "CREATE ROLE circle LOGIN PASSWORD 'circle';"
sudo -u postgres createdb -O circle circle
sudo -u postgres createdb -O circle circle_test
```

API, from `server/`:

```bash
go run ./cmd/api
```

It listens on [http://127.0.0.1:43123](http://127.0.0.1:43123). Override `ADDR`, `DATABASE_URL`, `REDIS_URL`, or `UPLOAD_DIR` if you need to.

App, from `mobile/`:

```bash
npm install
npm run web
```

The web preview listens on [http://127.0.0.1:43124](http://127.0.0.1:43124). For a phone, start Expo from `mobile/` with `npm start` and open the project in Expo Go. On an Android emulator, point the app at the host with `EXPO_PUBLIC_API_URL=http://10.0.2.2:43123`.

Use two accounts to try a circle: create an invite on My circle, accept it on People I watch.

## Tests

From `server/`, with Postgres and Redis running:

```bash
go test ./...
```

The tests cover the age boundary, sign-in, the supervisor cap, a stranger being refused someone's location, check confirmation, an unanswered check after 10 minutes, and live fan-out only to supervisors.
