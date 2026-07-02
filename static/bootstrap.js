// ===================== Initialize =====================
loadStats();
loadLogs();
loadAccounts();
loadWebhooks();
loadFilters();
loadProjects();
loadRules();

// Auto refresh
setInterval(loadLogs, 5000);
setInterval(loadStats, 10000);
