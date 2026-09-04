document.addEventListener("htmx:configRequest", (event) => {
  const csrfToken = document.cookie.split("; ").find((cookie) => cookie.startsWith("csrf_token="))?.split("=")[1]
  if (csrfToken) event.detail.headers["X-CSRF-Token"] = decodeURIComponent(csrfToken)
})

document.addEventListener("htmx:beforeRequest", (event) => {
  const form = event.detail.elt.closest("form")
  const target = form?.getAttribute("hx-target")
  if (target?.includes("error")) document.querySelector(target)?.replaceChildren()
})

document.addEventListener("htmx:beforeSwap", (event) => {
  if ([400, 401, 403, 404, 409, 500].includes(event.detail.xhr.status)) {
    event.detail.shouldSwap = true
    event.detail.isError = false
  }
})
