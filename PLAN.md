# Family Screen-Time Dashboard
## Project Plan and Product Specification

**Project status:** Planning  
**Project type:** Open-source, self-hosted application  
**Primary platform:** Android phones and tablets  
**Backend:** Go  
**Frontend:** SolidJS + Vite  
**Database:** PostgreSQL  
**Deployment:** Embedded frontend or separate frontend with reverse proxy

---

## 1. Project Overview

### 1.1 Purpose

The Family Screen-Time Dashboard is an open-source, self-hosted platform that allows individuals and families to monitor, aggregate, and review screen-time usage across multiple Android devices.

The system will collect usage data from Android devices, store it on a privately controlled server, and present it through a web dashboard. Family members can review their own usage, compare historical totals, and share selected information with authorized family members.

The project is intended for families who want greater awareness of their digital habits without relying on commercial services that may collect personal information for advertising or other unrelated purposes.

### 1.2 Core Principles

The project will follow these principles:

1. **Privacy first:** Usage data belongs to the people it describes and must not be sold or used for advertising.
2. **Self-hosted:** Users control where the server and database run.
3. **Open source:** The project should be transparent, auditable, and open to community contributions.
4. **Android-first:** Android usage collection is the primary data source for the initial release.
5. **Collector-driven design:** The server must be designed around the actual capabilities and payloads of Life Dashboard Companion.
6. **Family sharing with consent:** Family membership does not automatically grant unrestricted access to every member's data.
7. **Reliable reporting:** Repeated uploads, delayed synchronization, and multiple devices must be handled correctly.
8. **Flexible deployment:** The application must work both as a single binary and as separately deployed frontend and backend services.
9. **No enforcement by default:** The system focuses on monitoring and reporting rather than controlling or restricting device usage.

### 1.3 Primary Goals

- Collect screen-time usage data from Android phones and tablets.
- Aggregate usage across multiple devices belonging to the same person.
- Provide daily, weekly, and monthly usage reports.
- Support individual accounts and family groups.
- Allow authorized parents or guardians to view children's reports.
- Protect adult members' usage data through explicit sharing permissions.
- Provide secure device registration and revocation.
- Support self-hosted deployment with PostgreSQL.
- Allow users to export and delete their data.
- Provide a clear, documented integration with Life Dashboard Companion.

---

## 2. Scope

### 2.1 In Scope

- Android phone and tablet usage collection.
- Life Dashboard Companion integration.
- Webhook-based ingestion, subject to verification of collector capabilities.
- User registration and authentication.
- Device registration, naming, management, and revocation.
- Daily, weekly, and monthly reports.
- Aggregated usage across a person's devices.
- Application-level breakdowns where supported by the collector.
- Family creation and invitations.
- Parent/guardian access to authorized children's reports.
- Adult opt-in data sharing.
- Device synchronization status and stale-data indicators.
- Historical trends and data export.
- Data deletion and configurable retention.
- PostgreSQL storage.
- Go REST API.
- SolidJS and Vite frontend.
- Single-binary and separate-frontend deployment modes.
- Self-hosting documentation, backups, and restoration procedures.

### 2.2 Out of Scope for the Initial Release

The following features are explicitly excluded from the initial release:

- Blocking applications or enforcing screen-time limits.
- Website filtering.
- GPS or location tracking.
- Screenshots, keylogging, message collection, or call monitoring.
- iOS, desktop, or browser usage collection.
- Public leaderboards.
- Advertising or monetization based on personal usage data.
- AI-generated judgments about a person's behavior.
- Remote device control.
- Complex parental-control enforcement mechanisms.

These features may be evaluated in the future, but they must not compromise the project's privacy-first direction.

---

## 3. System Architecture

### 3.1 High-Level Architecture

The system consists of four main components:

1. **Android collector:** Life Dashboard Companion collects usage information from an Android device and sends supported data to the configured server.
2. **Go backend:** Receives usage data, authenticates devices, validates and stores records, manages accounts, enforces authorization, and generates reports.
3. **PostgreSQL database:** Stores accounts, devices, family relationships, usage records, permissions, and synchronization metadata.
4. **SolidJS frontend:** Provides the dashboard, account settings, family management, device management, and reports.

Conceptual data flow:

Android device → Life Dashboard Companion → HTTPS ingestion endpoint → Go backend → PostgreSQL → Go reporting API → SolidJS dashboard.

The precise integration contract must be confirmed before the ingestion schema and reporting calculations are finalized.

### 3.2 Technology Stack

| Component | Technology | Responsibility |
|---|---|---|
| Backend | Go | API, ingestion, authentication, authorization, aggregation |
| Frontend | SolidJS | Interactive dashboard and management UI |
| Frontend tooling | Vite | Development server and production builds |
| Database | PostgreSQL | Persistent storage and relational data |
| API | REST/JSON | Frontend and external integrations |
| Device ingestion | HTTPS webhook | Receive collector data |
| Embedded deployment | Go `embed` | Package frontend assets with the backend |
| Separate deployment | Static frontend + reverse proxy | Serve frontend and route API requests |
| Reverse proxy | Caddy, Nginx, or equivalent | Optional TLS termination and request routing |

Specific libraries and frameworks should be selected during technical design rather than fixed prematurely.

### 3.3 API Structure

Use versioned API routes to make future changes manageable.

Suggested structure:

- `/api/v1/auth/` — registration, login, logout, and session management.
- `/api/v1/users/` — account information and preferences.
- `/api/v1/devices/` — device registration and management.
- `/api/v1/families/` — family groups, membership, invitations, and sharing permissions.
- `/api/v1/reports/` — usage summaries, trends, and breakdowns.
- `/api/v1/data/` — export and data-management operations.
- `/webhooks/v1/` — device usage ingestion.

These routes are proposed conventions, not a finalized API contract.

The backend must remain fully functional as an API even when no frontend assets are embedded.

---

## 4. Android Collector Integration — First Priority

### 4.1 Integration Anchor

Life Dashboard Companion is the primary Android data collector for the initial version.

Repository: https://github.com/owen282000/life-dashboard-companion-app

The project must not assume that the collector supports a particular metric, authentication mechanism, synchronization schedule, or payload structure until those capabilities have been verified directly from its source code and documentation.

The server must accommodate the data the collector actually provides. A server-side feature cannot compensate for a collection capability that does not exist in the Android application.

### 4.2 Required Source Audit

Before implementing the final database schema or reports, investigate the collector's source code, documentation, configuration, and any existing integration examples.

Verify the following areas.

**Data collection**

- Which usage metrics are collected?
- Is usage reported per application, per device, or as a pre-aggregated total?
- Are durations reported in seconds, milliseconds, or another unit?
- Are timestamps, calendar dates, time zones, or reporting intervals included?
- Are foreground usage and screen-on time distinguishable?
- Is historical usage included in each upload?
- Are application identifiers, package names, or display names provided?

**Webhook integration**

- Can the destination URL be configured?
- What HTTP method and content type are used?
- What is the exact JSON payload structure?
- Can custom authentication headers be configured?
- Is HMAC signing supported, or is another authentication method available?
- Does the application expect a particular HTTP response?
- Are there payload size limits or batching rules?

**Synchronization behavior**

- How often does the application synchronize?
- Does synchronization require the application to be opened?
- What background execution restrictions apply?
- Does the application retry failed requests?
- Does it retain data while offline?
- Can requests be delivered more than once?
- Can older records arrive after newer records?
- How does it behave after a device restart or application update?

**Device identity and configuration**

- Does each payload contain a stable device identifier?
- Can the user configure the identifier?
- How are multiple devices distinguished?
- Can the same device be configured to send to a self-hosted endpoint?
- Which Android permissions and configuration steps are required?

### 4.3 Integration Deliverable

The source audit must produce a versioned **Android Integration Specification** containing:

- Supported collector version or commit.
- Verified setup and configuration instructions.
- Exact example payloads.
- A field-by-field payload description.
- Data units and timestamp semantics.
- Authentication capabilities and limitations.
- Retry, duplication, and synchronization behavior.
- Known Android background-execution limitations.
- A list of metrics supported by the collector.
- A list of metrics that are unavailable or ambiguous.
- Compatibility and migration considerations.

The specification should distinguish verified facts from assumptions and unresolved questions.

### 4.4 Integration Decision

After the audit, choose one of the following paths:

1. **Use the collector unchanged:** Preferred if it can securely deliver sufficient data to the server.
2. **Extend or fork the collector:** Necessary if a required capability is missing and can reasonably be added.
3. **Defer unsupported features:** Appropriate when a modification would be too invasive or unreliable.
4. **Design an alternative collector adapter:** A future extension point for other Android collection applications.

The initial release should not depend on unverified collector functionality.

---

## 5. Server Ingestion and Data Processing

### 5.1 Ingestion Responsibilities

The ingestion service will:

- Accept usage uploads from registered devices.
- Authenticate the submitting device.
- Validate payload structure and field types.
- Reject invalid or unauthorized submissions.
- Normalize incoming data into the internal data model.
- Associate each submission with the correct device and account.
- Handle duplicate, delayed, and out-of-order uploads.
- Record ingestion outcomes and synchronization timestamps.
- Return the response required by the collector's verified protocol.

The ingestion service must not trust a user ID supplied in an unauthenticated payload to determine data ownership.

### 5.2 Data Validation

Validation should cover:

- Required fields.
- Valid identifiers and timestamps.
- Supported units.
- Non-negative durations.
- Reasonable payload sizes.
- Allowed application and device associations.
- Valid reporting periods.
- Consistency between the payload and the registered device.

Invalid records should be rejected or quarantined according to a documented policy. Error responses must not reveal credentials or sensitive internal details.

### 5.3 Idempotency and Duplicate Prevention

The system must avoid counting the same usage data multiple times.

The implementation should determine whether the collector sends snapshots, incremental events, or cumulative records. The correct deduplication strategy depends on that distinction.

Potential approaches include:

- Stable source record identifiers.
- Unique constraints on device and source record identifiers.
- Idempotency keys for supported requests.
- Deterministic record keys based on verified source semantics.
- Replacing an existing aggregate when the collector sends an updated snapshot for the same reporting period.

Do not simply add every incoming duration to an account total. A retry or repeated snapshot could otherwise inflate reported usage.

### 5.4 Synchronization Health

Track enough metadata to help users understand whether their reports are current:

- Last successful ingestion.
- Last attempted ingestion, if available.
- Last accepted reporting period.
- Last ingestion error category.
- Collector or payload version, where available.

The UI should distinguish between a device with no recorded usage and a device that has not synchronized recently.

---

## 6. Data Model

The database schema should follow the verified collector contract and support future extensions without assuming unsupported metrics.

### 6.1 Proposed Core Entities

**Users**

Represents an individual account.

Suggested fields:

- ID.
- Email or other login identifier.
- Password hash or external authentication reference.
- Account status.
- Created and updated timestamps.

**Families**

Represents a group used to manage membership and sharing.

Suggested fields:

- ID.
- Name.
- Created timestamp.

**Family memberships**

Connects users to families and defines their roles and permissions.

Suggested fields:

- Family ID.
- User ID.
- Role.
- Membership status.
- Sharing settings.
- Joined timestamp.

**Invitations**

Supports controlled family membership.

Suggested fields:

- ID.
- Family ID.
- Invitation token hash.
- Intended recipient, where applicable.
- Intended role.
- Expiration timestamp.
- Status.

**Devices**

Represents a registered Android device.

Suggested fields:

- ID.
- Owner user ID.
- Display name.
- Collector type and version, where available.
- Credential reference or credential hash.
- Registration timestamp.
- Last successful synchronization.
- Revocation timestamp.

**Usage records**

Stores normalized usage data at the granularity supported by the collector.

Possible fields, subject to verification:

- ID.
- Device ID.
- Application identifier, if available.
- Reporting period or event timestamp.
- Usage duration.
- Source record identifier.
- Source payload version.
- Ingestion timestamp.

**Daily usage aggregates**

Optional derived records used to accelerate reports.

Possible fields:

- User ID.
- Device ID, if the aggregate is device-specific.
- Application identifier, if supported.
- Local reporting date.
- Aggregated duration.
- Calculation version or update timestamp.

Aggregates should be reproducible from the underlying records whenever practical.

### 6.2 Data Ownership

Every usage record must have a clear ownership path through its registered device to the responsible user.

The server must not allow a device to submit records on behalf of another user merely by changing an identifier in the payload.

Device reassignment must be an explicit, authorized operation with a documented policy for historical records.

### 6.3 Time Zones and Reporting Periods

Reports should use a clearly defined time-zone policy.

The project must decide:

- Whether reporting dates use each user's preferred time zone.
- How daylight-saving transitions are handled where applicable.
- Whether the original source timestamps are retained.
- How late-arriving data changes previously generated reports.
- Whether users can change the time zone used for historical reports.

Raw timestamps should be retained when available. Aggregation should not silently mix reporting periods from different time zones.

### 6.4 Multiple Devices and Overlapping Usage

The dashboard should distinguish between:

- The sum of reported usage durations across devices.
- Usage duration attributed to individual devices.
- Any estimate of unique elapsed time, if reliable source data makes such an estimate possible.

If two devices report simultaneous usage, summing both durations can exceed the elapsed time in a calendar day. The product must not present a sum of device durations as unique personal screen time unless it can reliably calculate that metric.

---

## 7. User Accounts and Device Management

### 7.1 User Accounts

Users should be able to:

- Register an account.
- Log in and log out.
- Manage account settings.
- View their own reports.
- Register and manage their Android devices.
- Manage family membership and sharing permissions.
- Export or delete their data.

### 7.2 Device Registration

A registered device should have its own revocable credential.

Recommended setup flow:

1. A user logs in to the dashboard.
2. The user registers a new device.
3. The server creates a device record and device-specific credential.
4. The user follows the documented Life Dashboard Companion configuration instructions.
5. The device sends a test upload.
6. The server validates and accepts the upload.
7. The dashboard shows the device as connected after successful ingestion.

The exact setup steps depend on the collector's verified configuration options.

Credentials should be shown only when needed for initial setup. They must not be stored in plaintext unless there is a compelling, documented requirement.

### 7.3 Device Management

Users should be able to:

- Rename devices.
- View synchronization status.
- Review the last successful upload.
- Revoke a device's credential.
- Remove devices from their account.
- Reassign devices through an explicit, authorized process.

Revocation must prevent further accepted uploads using the revoked credential.

---

## 8. Usage Reports and Dashboard

### 8.1 Main Dashboard

The dashboard should provide a clear overview of recent usage, including:

- Today's reported usage.
- Usage over the last seven days.
- Monthly usage totals.
- Comparison with previous periods.
- Per-device summaries.
- Per-application breakdowns when available.
- Synchronization status.

Unavailable metrics should be omitted or clearly marked as unsupported rather than represented with invented or inferred values.

### 8.2 Reporting Periods

The initial release should support:

- Daily reports.
- Weekly reports.
- Monthly reports.

The report interface should specify the period being displayed and the time zone used for its boundaries.

### 8.3 Cross-Device Aggregation

When a person has multiple registered devices, reports should aggregate their usage according to the verified source data and documented calculation rules.

The interface should allow users to distinguish:

- Total reported device usage.
- Usage by device.
- Usage by application, if available.

The aggregation logic must be consistent across dashboard views, API responses, and exported reports.

### 8.4 Historical Trends

Provide basic charts that help users understand changes over time.

Examples include:

- Daily usage trends.
- Weekly averages.
- Monthly comparisons.
- Application usage distribution.
- Device-level comparisons.

Advanced analytics should follow only after the core metrics are validated.

### 8.5 Export

Users should be able to export their own usage records and reports in a documented format, such as CSV or JSON.

Exports must respect the same authorization rules as the dashboard and must not include other family members' data without permission.

---

## 9. Family Management and Sharing

### 9.1 Family Groups

Users should be able to create a family group and invite other members.

Family groups provide a way to manage sharing and access; they are not a blanket authorization mechanism.

### 9.2 Roles

The initial role model may include:

- **Family administrator:** Manages family settings and invitations.
- **Parent or guardian:** Can view reports for children when authorized.
- **Adult member:** Controls sharing of their own data.
- **Child member:** Has an age-appropriate account and access model, subject to the project's guardian and consent rules.

Roles must be enforced by the backend rather than only by hiding frontend controls.

### 9.3 Adult Privacy

Adult family members' usage data must remain private unless they explicitly grant access.

Membership in a family must not automatically grant another adult access to detailed application usage.

Where practical, sharing permissions should distinguish between:

- No sharing.
- Summary-only sharing.
- Detailed report sharing, including application-level information when available.

The exact granularity should reflect the actual data model and remain understandable to users.

### 9.4 Children and Guardianship

The project must define how child accounts are created, how guardian relationships are established, and how those relationships can be changed.

Before implementation, decide:

- Who can create a child account.
- How a guardian's authority is verified within the application.
- Whether child members can view their own reports.
- Which reports guardians can access.
- How permissions change when a child reaches adulthood.
- How family separation or membership disputes are handled.

The application should avoid claiming that its internal role system establishes legal guardianship.

### 9.5 Leaving a Family

The product must document what happens when a member leaves a family.

At minimum, leaving a family should revoke future access based on that membership. Historical ownership and deletion policies must be defined separately so that membership changes do not silently transfer or erase personal data.

---

## 10. Security and Privacy

### 10.1 Authentication

Account authentication should use established security practices.

Recommended requirements:

- Secure password hashing if passwords are used.
- HTTPS for all remote access.
- Secure, HTTP-only session cookies for browser sessions.
- Appropriate SameSite settings and CSRF protections where applicable.
- Session expiration and logout invalidation.
- Rate limiting for authentication endpoints.
- Safe account recovery procedures.

### 10.2 Device Authentication

Each registered device should have an independent, revocable credential.

Use the strongest authentication mechanism the collector can actually support.

If HMAC signing is supported, evaluate it for request integrity and replay protection. If it is not supported, document the alternative and its limitations rather than assuming HMAC can be configured solely on the server.

Device credentials must not grant access to the general user API or other devices' data.

### 10.3 Authorization and Tenant Isolation

Every protected API request must be authorized on the server.

The system must verify:

- The user owns the requested device or has the appropriate management permission.
- The user can access the requested report.
- Family sharing permissions authorize access to another person's data.
- The requested records belong to the correct account or authorized family relationship.

Changing a user ID, device ID, or family ID in a URL must never expose another person's data.

### 10.4 Data Minimization

Collect and retain only information necessary for the product's stated functionality.

Do not collect GPS, messages, screenshots, keystrokes, or other unrelated personal information.

Avoid third-party analytics and tracking scripts in the dashboard. If optional telemetry is ever introduced, it must be disabled by default or follow a clearly documented, privacy-preserving opt-in policy.

### 10.5 Data Retention and Deletion

Provide a documented retention policy and allow users to delete their data.

Before release, define:

- Default retention duration.
- Whether raw records and aggregates have different retention periods.
- How deletion affects reports and derived aggregates.
- How device removal affects historical usage.
- How deletion propagates to backups.
- Whether and how deletion can be delayed until backup expiration.

Deletion behavior must be explicit and predictable.

### 10.6 Audit Logging

Record security-relevant events where necessary, such as:

- Device registration and revocation.
- Family membership changes.
- Sharing permission changes.
- Account security changes.
- Data export or deletion operations.

Audit logs should avoid storing raw usage payloads, passwords, tokens, or unnecessary personal information.

### 10.7 Backup and Recovery

Document how to:

- Back up PostgreSQL.
- Protect backup files.
- Restore the database.
- Restore the application configuration.
- Rotate credentials after a suspected compromise.
- Test recovery procedures.

Self-hosting does not automatically guarantee privacy or security. Administrators remain responsible for securing their server, network, backups, and credentials.

---

## 11. Deployment Modes

Both deployment modes must use the same backend behavior and API contract.

### 11.1 Mode A: Single Binary

Package the compiled SolidJS frontend assets into the Go executable using `embed`.

The resulting application should:

- Serve the web interface.
- Expose the REST API.
- Receive webhook uploads.
- Connect to PostgreSQL.
- Support configuration through environment variables or a configuration file.
- Run without a separate frontend web server.

The database remains a separate service unless a future design explicitly introduces another storage option.

### 11.2 Mode B: Separate Frontend and Backend

Build the frontend as static assets and serve them through Caddy, Nginx, or another compatible web server.

The Go backend runs independently and exposes the API and ingestion endpoints.

The reverse proxy should:

- Serve the frontend.
- Forward `/api/v1/` requests to the backend.
- Forward `/webhooks/v1/` requests to the backend.
- Handle HTTPS where appropriate.
- Apply suitable request size and timeout limits.
- Avoid exposing unnecessary internal services.

### 11.3 Frontend Routing

The frontend should use relative API paths by default so that a same-origin reverse proxy can route requests without requiring a hard-coded hostname.

If hosting the frontend under a subpath is supported, configure the frontend base path explicitly and test client-side routing and asset URLs.

### 11.4 Configuration

Configuration should cover the essential operational settings, including:

- Database connection.
- HTTP listener.
- Public application URL where required.
- Session and device credential secrets.
- Secure cookie settings.
- Frontend serving mode.
- Logging level.
- Retention policy.
- Optional trusted proxy configuration.

Secrets must not be committed to source control or printed in logs.

### 11.5 Operational Documentation

Provide documentation for:

- Local development.
- Production deployment.
- Environment configuration.
- Database initialization and migrations.
- Reverse proxy setup.
- HTTPS.
- Backups and restoration.
- Updating the application.
- Troubleshooting collector ingestion.
- Updating supported collector versions.

---

## 12. Development Roadmap

Development should follow dependency order. The Android integration audit is the first milestone because the collector determines which metrics, synchronization behaviors, and security mechanisms the rest of the system can reliably support.

### Phase 1 — Collector Audit and Integration Specification

**Objective:** Establish exactly what Life Dashboard Companion can provide.

Tasks:

- Inspect source code and documentation.
- Verify webhook configuration and authentication capabilities.
- Capture and document actual payload examples.
- Determine available metrics and units.
- Investigate synchronization, retries, offline behavior, and duplicate delivery.
- Verify device identity and multi-device support.
- Document setup instructions and limitations.
- Decide whether the collector can be used unchanged or needs modification.

**Deliverable:** A verified Android Integration Specification and an explicit compatibility decision.

**Exit criteria:** The team can explain which reports can be generated reliably from actual collector data.

### Phase 2 — Technical Foundation and Data Model

**Objective:** Establish the backend and storage foundation.

Tasks:

- Create the Go project structure.
- Establish database migrations.
- Define user, device, family, and usage entities.
- Define time-zone and duration semantics.
- Define the internal ingestion contract.
- Implement configuration and structured logging.
- Define API versioning and error response conventions.
- Add automated tests and development tooling.

**Deliverable:** A runnable backend connected to PostgreSQL with documented schemas and API conventions.

### Phase 3 — Device Ingestion

**Objective:** Reliably receive and store usage data.

Tasks:

- Implement the webhook endpoint.
- Implement device authentication.
- Validate collector payloads.
- Normalize supported records.
- Implement idempotency and duplicate prevention.
- Handle retries and out-of-order records.
- Record synchronization metadata.
- Add ingestion integration tests.

**Deliverable:** Registered Android devices can submit valid data securely and repeatedly without inflating usage totals.

### Phase 4 — Accounts and Device Management

**Objective:** Establish personal ownership and device control.

Tasks:

- Implement registration and login.
- Implement session management.
- Implement device registration and credential issuance.
- Implement device listing, renaming, and revocation.
- Add test-upload or connection-status feedback.
- Enforce device ownership in the API.

**Deliverable:** Users can securely manage their own accounts and multiple devices.

### Phase 5 — Reports and Aggregation

**Objective:** Provide accurate, useful usage summaries.

Tasks:

- Implement daily aggregation.
- Implement weekly and monthly reports.
- Add per-device summaries.
- Add per-application breakdowns if supported.
- Define late-arriving-data behavior.
- Add historical trend charts.
- Add export.
- Test aggregation across multiple devices and time zones.

**Deliverable:** Users can review accurate reports derived from verified collector data.

### Phase 6 — Family Groups and Sharing

**Objective:** Support family reporting without unrestricted access.

Tasks:

- Implement family creation.
- Implement invitation and membership workflows.
- Implement role and permission checks.
- Implement parent/guardian report access.
- Implement adult opt-in sharing.
- Enforce family and account isolation.
- Define member departure and historical-data policies.
- Add authorization tests.

**Deliverable:** Authorized family members can view permitted reports while other members' data remains private.

### Phase 7 — Frontend and Deployment

**Objective:** Deliver a usable dashboard in both deployment modes.

Tasks:

- Build the SolidJS dashboard.
- Implement account and device management screens.
- Implement reports and charts.
- Implement family and sharing screens.
- Add responsive layouts and accessibility improvements.
- Add embedded frontend support in Go.
- Add separate static frontend deployment.
- Test API routing, frontend routing, configuration, and HTTPS.

**Deliverable:** Both supported deployment modes operate against the same backend API.

### Phase 8 — Security, Reliability, and Release

**Objective:** Prepare a dependable first public release.

Tasks:

- Perform an authorization and tenant-isolation review.
- Test device credential revocation.
- Review session security and request validation.
- Test duplicate and delayed uploads.
- Test database backups and restoration.
- Verify export and deletion behavior.
- Document known collector limitations.
- Publish installation and upgrade instructions.
- Add contribution guidelines and release notes.
- Establish a process for reporting vulnerabilities.

**Deliverable:** A documented, tested MVP suitable for self-hosted use.

---

## 13. MVP Acceptance Criteria

The first release is considered complete when all applicable criteria below are satisfied.

### Collector Integration

- [ ] Life Dashboard Companion integration is documented and verified.
- [ ] Supported collector versions and limitations are documented.
- [ ] At least one Android device can submit data successfully.
- [ ] The server validates incoming payloads.
- [ ] Device authentication works with the collector's supported capabilities.
- [ ] Synchronization status is visible.

### Accounts and Devices

- [ ] Users can register and log in securely.
- [ ] A user can register at least two Android devices.
- [ ] Each device has independently revocable credentials.
- [ ] Users can rename and revoke their devices.
- [ ] Revoked credentials can no longer submit accepted data.

### Data and Reporting

- [ ] Device usage is stored persistently in PostgreSQL.
- [ ] Repeated uploads do not inflate usage totals.
- [ ] Delayed or out-of-order uploads are handled according to documented rules.
- [ ] Daily reports are available.
- [ ] Weekly reports are available.
- [ ] Monthly reports are available.
- [ ] Usage can be aggregated across one person's devices.
- [ ] Unsupported metrics are not fabricated.
- [ ] Time-zone behavior is documented and tested.
- [ ] Users can export their own data.

### Family and Privacy

- [ ] Users can create a family and invite members.
- [ ] Parent/guardian access is enforced by backend authorization.
- [ ] Adult members' data remains private unless sharing is granted.
- [ ] Users cannot access another account's records by changing URL identifiers.
- [ ] Family membership changes update access appropriately.
- [ ] Users can revoke device access and initiate data deletion.

### Deployment and Operations

- [ ] The embedded frontend deployment works.
- [ ] The separate frontend deployment works.
- [ ] Both modes use the same API contract.
- [ ] PostgreSQL backup and restoration procedures are documented.
- [ ] Production configuration does not require secrets to be committed to source control.
- [ ] HTTPS deployment guidance is available.
- [ ] Automated tests cover critical ingestion and authorization paths.

---

## 14. Testing Strategy

### 14.1 Unit Tests

Test isolated business logic, including:

- Payload validation.
- Duration and timestamp conversion.
- Aggregation calculations.
- Reporting period boundaries.
- Permission evaluation.
- Idempotency key generation where applicable.

### 14.2 Integration Tests

Test interactions among:

- Go backend and PostgreSQL.
- Device registration and authentication.
- Webhook ingestion and persistence.
- Raw usage records and aggregates.
- Account ownership and report queries.
- Family membership and sharing permissions.

### 14.3 Collector Compatibility Tests

Maintain representative payload fixtures from supported collector versions.

For each supported version, test:

- Valid payload acceptance.
- Invalid payload rejection.
- Repeated upload behavior.
- Delayed record handling.
- Missing optional fields.
- Payload schema changes.

Fixtures must not contain real family usage data or credentials.

### 14.4 Security Tests

Verify that:

- Unauthenticated users cannot access protected endpoints.
- One user cannot access another user's devices.
- Devices cannot submit data for another account.
- Family members cannot bypass sharing permissions.
- Adult reports remain private by default.
- Revoked device credentials are rejected.
- Export and deletion endpoints enforce ownership.
- Authentication and ingestion endpoints resist basic abuse and excessive request sizes.

### 14.5 End-to-End Tests

Run the complete flow:

1. Create an account.
2. Register an Android device.
3. Configure the collector.
4. Submit usage data.
5. Verify ingestion status.
6. View the resulting report.
7. Register a second device.
8. Verify cross-device aggregation.
9. Repeat an upload and confirm totals remain correct.
10. Create a family and verify authorized sharing.
11. Verify unauthorized access is denied.
12. Revoke a device and confirm subsequent uploads fail.

---

## 15. Project Structure

A possible repository layout is:

```text
family-screen-time/
├── cmd/
│   └── server/
├── internal/
│   ├── auth/
│   ├── config/
│   ├── devices/
│   ├── families/
│   ├── ingestion/
│   ├── reports/
│   ├── storage/
│   └── users/
├── migrations/
├── web/
│   ├── src/
│   ├── public/
│   ├── package.json
│   └── vite.config.ts
├── docs/
│   ├── architecture.md
│   ├── android-integration.md
│   ├── api.md
│   ├── privacy.md
│   ├── security.md
│   └── deployment.md
├── deploy/
│   ├── docker-compose.yml
│   └── Caddyfile
├── tests/
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── CONTRIBUTING.md
├── LICENSE
└── PROJECT_PLAN.md
```

This is a proposed starting structure. Adjust it to fit the eventual implementation without mixing business logic, transport handling, and persistence unnecessarily.

The project may later separate the frontend into its own repository if that improves maintenance, but a single repository is a reasonable initial choice.

---

## 16. Open-Source and Community Strategy

### 16.1 Documentation

The repository should include:

- A concise project overview.
- A feature and scope list.
- Installation instructions.
- Configuration reference.
- Android collector setup instructions.
- API documentation.
- Data and privacy explanations.
- Backup and restoration guidance.
- Troubleshooting.
- Known limitations.

### 16.2 Contribution Guidelines

Contributors should understand:

- How to run the project locally.
- How to run tests.
- How database migrations are managed.
- How collector compatibility is tested.
- How security issues should be reported.
- Which privacy principles contributions must preserve.

### 16.3 Licensing

Choose a license before the first public release and verify compatibility with all dependencies and any reused collector code.

The project's license should clearly cover its own code without implying ownership of third-party software or granting rights the project does not possess.

### 16.4 Future Extensibility

After the MVP is stable, potential additions include:

- Additional Android collectors.
- More flexible report periods.
- Configurable retention.
- Better import and export tools.
- Localization.
- Optional synchronization alerts.
- Improved accessibility.
- Additional deployment examples.
- A documented adapter interface for future data sources.

New integrations should use a common internal data model while preserving the distinctions between source metrics and calculated summaries.

---

## 17. Decisions Required Before Final Technical Design

These decisions must be resolved before finalizing the database schema and report semantics.

| Decision | Why it matters |
|---|---|
| Definition of screen time | Device usage, foreground application duration, and screen-on time are not necessarily equivalent. |
| Collector payload semantics | Determines normalization, idempotency, and report calculations. |
| Device synchronization behavior | Determines retry handling, stale-data detection, and reconciliation. |
| Cross-device calculation | Determines whether reports show summed duration or another metric. |
| Time-zone policy | Determines daily and monthly boundaries. |
| Child account creation | Determines invitation, guardian, and account lifecycle rules. |
| Adult sharing granularity | Determines whether summaries and detailed application reports can be shared independently. |
| Shared devices | Determines whether a device can have multiple owners and how usage is attributed. |
| Device reassignment | Determines who retains historical records after ownership changes. |
| Family departure | Determines historical access and ownership after a member leaves. |
| Data retention | Determines database growth, deletion, and backup requirements. |
| Authentication capabilities | Determines the strongest practical device authentication supported by the collector. |

These decisions should be recorded in architecture decision records when they have significant technical consequences.

---

## 18. Definition of Success

The project succeeds when a family can deploy the application on its own server, connect Android devices through a verified collector integration, and obtain accurate usage reports without surrendering control of its data to a commercial monitoring service.

The initial release should prioritize trustworthy collection, accurate aggregation, secure authorization, and understandable privacy controls over a large number of features.

The most important dependency is the Android collector. Until its actual capabilities and payload format have been verified, the database schema, aggregation rules, and detailed report requirements remain provisional.

**The first development milestone is therefore the Life Dashboard Companion source audit and integration specification—not the dashboard UI.** Once the collector contract is understood, the remaining architecture can be built on a reliable foundation.

