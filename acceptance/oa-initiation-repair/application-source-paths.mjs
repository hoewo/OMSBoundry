export function applicationRelativePath(app, aliases = {}) {
  const module = aliases[app] || app;
  if (module === 'unified-workbench') return 'frontend/oms-workbench';
  if (module === 'gateway' || module === 'identity-access') return 'backend/platform/' + module;
  return 'backend/services/' + module;
}

export function applicationSourcePath(root, app, aliases = {}) {
  return root + applicationRelativePath(app, aliases);
}
