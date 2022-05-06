function getFilename(file) {
    return file.webkitRelativePath || file.fileName || file.name // Some confusion in different versions of Firefox
}

let r = undefined

function initResumableUploader(target, maxFiles, allowedExtensions, pollStatus) {
    r = new Resumable({
        target: target,
        chunkSize: 16 * 1024 * 1024,
        simultaneousUploads: 2,
        query: {},
        generateUniqueIdentifier: function (file, event) {
            let relativePath = getFilename(file)
            let size = file.size
            let utf8 = unescape(encodeURIComponent(relativePath));
            let encoded = ""
            for (let i = 0; i < utf8.length; i++) {
                encoded += utf8.charCodeAt(i).toString()
            }

            return  size + "-" + encoded
        },
        maxFiles: maxFiles,
        testChunks: true,
    })

    if (r.support) {
        document.getElementById("content-resumable").hidden = false
    } else {
        document.getElementById("content-legacy").hidden = false
    }

    r.assignBrowse(document.getElementById("resumable-drop"));
    r.assignDrop(document.getElementById("resumable-drop"));


    let progressBarsContainer = document.getElementById("progress-bars-container-resumable")

    function addFile(file) {
        let progressBar = document.createElement("progress");
        progressBar.max = 1
        progressBar.value = 0
        let progressText = document.createElement("span");
        progressText.style.fontWeight = "bold"
        progressText.style.fontSize = "90%"
        progressText.style.textShadow = "0 1px 1px rgba(0, 0, 0, 0.08)"
        progressBarsContainer.appendChild(progressText)
        progressBarsContainer.appendChild(progressBar)

        progressText.innerHTML = `${getFilename(file)}<br>Queued for upload...`

        file.progressText = progressText
        file.progressBar = progressBar
        file.uploadStartTime = null
    }

    function updateFileProgress(file) {
        if (file.uploadStartTime === null) {
            file.uploadStartTime = performance.now()
        }
        let percent_complete = file.progress() * 100

        let currentUploadSpeed = ((file.progress() * file.size) / (performance.now() - file.uploadStartTime)) * 1000 // why do i need to multiply by 1000 here?

        file.progressText.innerHTML = `${getFilename(file)}<br>Progress: ${percent_complete.toFixed(3)}% Upload speed: ${sizeToString(currentUploadSpeed)}/s`

        file.progressBar.value = file.progress()
    }

    function fileError(file, message) {
        try {
            const obj = JSON.parse(message);
            if (obj["status"] === 409) {
                file.progressText.style.color = "orange"
            } else {
                file.progressText.style.color = "red"
            }
            file.progressText.innerHTML = `${getFilename(file)}<br>Upload failed!<br>Request status: ${obj["status"]} - ${friendlyHttpStatus[obj["status"]]}<br>Server response: ${obj["message"]}`
        } catch (e) {
            file.progressText.innerHTML = `${getFilename(file)}<br>Upload failed!<br>Server response: ${message}`
        }
    }

    function updateFileSuccess(file, message) {
        if(pollStatus === true) {
            try {
                const obj = JSON.parse(message);
                file.progressText.innerHTML = "Upload finished, fetching status..."
                file.progressText.innerHTML += "<br><img src=/static/fpfss-spinner.gif>"
                file.progressBar.value = 1
                file.intervalID = setInterval(pollUploadStatus, 500, file, obj["temp_name"]);

            } catch (e) {
                file.progressText.innerHTML = `${getFilename(file)}<br>The submission is being processed.<br>Server response: ${message}`
            }
        } else {
            try {
                const obj = JSON.parse(message);
                file.progressText.innerHTML = `${getFilename(file)}<br>Upload successful. <a href="/web${obj["url"]}">View</a>`
                file.progressBar.value = 1
            } catch (e) {
                file.progressText.innerHTML = `${getFilename(file)}<br>Upload successful.<br>Server response: ${message}`
            }
        }
    }

    r.on("fileSuccess", updateFileSuccess);
    r.on("fileProgress", updateFileProgress);
    r.on("filesAdded", function (array) {
        for (let i = 0; i < array.length; i++) {
            let goodExtension = false
            if (allowedExtensions.length === 0) {
                goodExtension = true
            } else {
                for (let j = 0; j < allowedExtensions.length; j++) {
                    if (getFilename(array[i]).endsWith(allowedExtensions[j])) {
                        goodExtension = true
                        break
                    }
                }
            }
            if (!goodExtension) {
                alert('Error : Incorrect file type.')
                return
            }
        }
        for (let i = 0; i < array.length; i++) {
            addFile(array[i])
        }
    });
    r.on("fileRetry", function (file) {
        console.debug("fileRetry", file);
    });
    r.on("fileError", fileError);
    r.on("uploadStart", function () {
        console.debug("upload started");
    });
    r.on("complete", function () {
        console.debug("all uploads complete");
    });
    // r.on("progress", function () {
    //     console.debug("progress");
    // });
    r.on("error", function (message, file) {
        fileError(file, message)
    });
    r.on("pause", function () {
        console.debug("upload paused");
    });
    r.on("cancel", function () {
        console.debug("upload canceled");
    });
}

function startUpload() {
    r.upload()
}

function pauseUpload() {
    r.pause()
}

function cancelUpload() {
    document.getElementById("progress-bars-container-resumable").innerHTML = ""
    r.cancel()
}

function pollUploadStatus(file, tempName) {
    let request = new XMLHttpRequest()
    request.open("GET", `/api/upload-status/${tempName}`, true)

    request.addEventListener("loadend", function () {
        if (request.status !== 200) {
            file.progressText.innerHTML = `${getFilename(file)}<br>There was an error fetching the upload status, trying again...`
            return
        }
        
        let status = null
        try {
            status = JSON.parse(request.response)["status"]
            file.progressText.innerHTML = `${getFilename(file)}<br>Status: ${status["status"]}`
            if (status["message"] !== null) {
                file.progressText.innerHTML += `<br>Message: ${status["message"]}`
            }
            if (status["submission_id"] !== null) {
                file.progressText.innerHTML += `<br><a href="/web/submission/${status["submission_id"]}">View</a>`
                clearInterval(file.intervalID)
            }
            if (status["status"] == "failed") {
                clearInterval(file.intervalID)
            } else {
                file.progressText.innerHTML += "<br><img src=/static/fpfss-spinner.gif>"
            }
        } catch (err) {
            console.error(err)
            file.progressText.innerHTML = `${getFilename(file)}<br>There was an error fetching the upload status, trying again...`
            return
        }
    })

    request.send()
}