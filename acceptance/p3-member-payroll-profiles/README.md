# P3 member and payroll profile acceptance

`run-isolated` starts PostgreSQL 18, People & Groups, and Payroll on an isolated
Docker network. It proves the first member/profile slice through the public
versioned HTTP APIs rather than by sharing databases:

- create, offboard, and rehire one Person while preserving employment history;
- create and revise compensation and payment profiles through Payroll;
- keep payment account numbers masked on ordinary reads and retain account history;
- allow a compensation revision while the Person is inactive, because employment
  status is not a Payroll eligibility rule;
- prove later People revisions do not rewrite Payroll snapshots; and
- reject anonymous access and a profile for an unknown Person.

Run from the repository root:

```sh
./acceptance/p3-member-payroll-profiles/run-isolated
```

The script creates only temporary containers, images, a network, and a temporary
directory, then removes them on exit. It does not deploy D1 or public environments.
