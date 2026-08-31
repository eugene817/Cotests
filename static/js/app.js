document.body.addEventListener("htmx:configRequest", (event) => {
  const csrfToken = document.cookie.split("; ").find((cookie) => cookie.startsWith("csrf_token="))?.split("=")[1]
  if (csrfToken) event.detail.headers["X-CSRF-Token"] = decodeURIComponent(csrfToken)
})

document.body.addEventListener("htmx:beforeSwap", (event) => {
  if ([400, 401, 403, 409].includes(event.detail.xhr.status)) {
    event.detail.shouldSwap = true
    event.detail.isError = false
  }
})
