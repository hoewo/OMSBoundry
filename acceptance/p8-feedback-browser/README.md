# P8 Feedback cross-origin browser acceptance

This low-resource acceptance test loads the production `oms_phase1_feedback.js` in a real headless Chromium browser. Two temporary loopback HTTP servers use different random ports, so the Workbench host and SDK fixture have different origins without starting Docker or any OMS service.

It verifies:

- one reusable cross-origin iframe and the production postMessage bridge;
- protected runtime configuration, project 115, submit/status opens and unread count;
- rejection of a same-page forged SDK message;
- validated close and session refresh messages;
- logout cleanup;
- undiscoverable `403` capability denial;
- local `503` degradation while the rest of the OMS page remains interactive.

The fixture uses an obviously synthetic key and never contacts the external Feedback project. Passing this test proves the local browser bridge only; it does not prove project 115 notifications, real credentials, tenant isolation, attachments, replies or external origin allowlisting.

Run with an installed Playwright module. For the Codex bundled runtime on this workstation:

```sh
PLAYWRIGHT_MODULE_PATH=/absolute/path/to/playwright/index.mjs \
BROWSER_EXECUTABLE='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' \
make p8-feedback-browser-test
```

Both servers and the browser are closed in `finally`, including failed runs.

