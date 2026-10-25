# P4 payroll execution acceptance

`run-isolated` builds and starts People & Groups, Payroll, and Money Flow Ledger
against three separate databases in one temporary PostgreSQL 18 container. It
proves the versioned HTTP path from member facts to an immutable payroll run and
then to actual Money payment/allocation facts.

The acceptance covers:

- compensation and payment profile snapshots plus an explicitly imported test
  contribution scheme;
- payroll import preview/commit, review, and exact integer contribution values;
- batch payment partial success where one member is paid and a missing account
  fails without rolling back the successful Money facts;
- a second partial payment, explicit overage confirmation, and exact Money
  allocation/unallocated amounts; and
- idempotent owner/mapping prerequisites and immutable cross-owner references.

Run from the repository root:

```sh
./acceptance/p4-payroll-execution/run-isolated
```

The scheme is deliberately labelled as a test fixture. It does not claim to be
the current statutory rate for any real jurisdiction. The script does not deploy
D1 or any public environment and removes its own processes and container on exit.
