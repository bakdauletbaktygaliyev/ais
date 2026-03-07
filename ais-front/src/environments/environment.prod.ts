export const environment = {
    production: true,
    apiUrl: '/api/v1',
    wsUrl: `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws`,
};