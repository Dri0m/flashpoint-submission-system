function sendDelete(url) {
    if (confirm(`Are you sure you want to (soft) delete '${url}'?`)) {
        let request = new XMLHttpRequest()
        request.open("DELETE", url, false)

        try {
            request.onload = function () {
                if (request.status !== 204) {
                    alert(`failed to delete '${url}' - status ${request.status}`)
                } else {
                    alert(`delete of '${url}' successful`)
                    location.reload()
                }
            };
            request.send()
        } catch (err) {
            alert(`failed to delete '${url}' - exception '${err.message}'`)
        }
    }
}