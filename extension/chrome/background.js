// La icona obre el panell lateral: el xat amb els vents viu al costat de la pàgina.
chrome.runtime.onInstalled.addListener(() => {
  chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true }).catch(() => {});
});
