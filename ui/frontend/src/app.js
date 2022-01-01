import Alpine from 'alpinejs'

window.Alpine = Alpine
const apiEndpoint = process.env.DEV_API_ENDPOINT || '/api/v1'

Alpine.data('app', () => ({
  websites: [],
  showModal: false,
  modalTitle: "",
  log: "",
  init() {
    fetch(apiEndpoint + '/websites')
    .then(response => response.json())
    .then(data => {
      this.websites = data
    })
    .catch(error => {
      console.error('failed to fetch websites', error);
    });
  },
  getLog(ns, name) {
    this.showModal = true
    this.modalTitle = ns + "/" + name
    fetch(apiEndpoint + '/logs/' + ns + '/' + name)
    .then(response => response.text())
    .then(data => {
      this.log = data
    })
    .catch(error => {
      console.error('failed to fetch logs', error);
    });
  }
}));

Alpine.start()
