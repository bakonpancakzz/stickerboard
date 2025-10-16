//@ts-check

//  ██
//  ██                      ██    ██
//  ██ ██ ██  ██ ██ ██   ██ ██ ██ ██ ██   
//  ██    ██  ██    ██   ██ ██ ██ ██ ██
//  ██ ██ ██  ██ ██ ██      ██ ██ ██
//            ██               ██
//            ██               

(() => {
    const blank_pixel = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABAQMAAAAl21bKAAAAA1BMVEUAAACnej3aAAAAAXRSTlMAQObYZgAAAApJREFUCNdjYAAAAAIAAeIhvDMAAAAASUVORK5CYII="

    /** @param {any} s */
    const $ = s => document.querySelector(s)

    /** @param {string} s */
    const safe_url = s => {
        try {
            new URL(s)
            return true
        } catch (_) {
            return false
        }
    }

    /** @param {Blob} f */
    const file_url = f => new Promise((o, r) => {
        const reader = new FileReader()
        reader.onerror = e => r(e)
        reader.onload = () => o(reader.result)
        reader.readAsDataURL(f)
    })

    const
    /** @type {HTMLImageElement?} */  elemCanvas = $("#canvas"),
    /** @type {HTMLImageElement?} */  elemPreview = $("#preview"),
    /** @type {HTMLInputElement?} */  formFile = $("#form-file"),
    /** @type {HTMLInputElement?} */  formX = $("#form-x"),
    /** @type {HTMLInputElement?} */  formY = $("#form-y"),
    /** @type {HTMLInputElement?} */  formS = $("#form-s"),
    /** @type {HTMLInputElement?} */  formUserName = $("#form-username"),
    /** @type {HTMLInputElement?} */  formUserURL = $("#form-userurl"),
    /** @type {HTMLTextAreaElement?} */  formComment = $("#form-comment"),
    /** @type {HTMLParagraphElement?} */  formError = $("#form-error"),
    /** @type {HTMLButtonElement?} */  formSubmit = $("#form-submit"),
    /** @type {HTMLDivElement?} */  paneStickers = $(".pane-stickers"),
    /** @type {HTMLDivElement?} */  paneUploader = $(".pane-uploader"),
    /** @type {HTMLButtonElement?} */  buttonSwap = $("#pane-swap")

    let CANVAS_WIDTH = 854, CANVAS_HEIGHT = 480, CANVAS_STICKER_MAX_HEIGHT = 240, CANVAS_STICKER_MAX_HEIGHT_START = 160
    let scale = 1, ox = 0, oy = 0, iw = 0, ih = 0, click = false, busy = false, preview = false

    // Pane Swapping
    if (buttonSwap) buttonSwap.onclick = (() => {
        let viewUploader = false
        return () => {
            if (!paneStickers || !paneUploader) return
            if (viewUploader) {
                buttonSwap.textContent = "Create Sticker"
                paneStickers.removeAttribute("hidden")
                paneUploader.setAttribute("hidden", "true")
                canvas_clear()
            } else {
                buttonSwap.textContent = "Cancel"
                paneStickers.setAttribute("hidden", "true")
                paneUploader.removeAttribute("hidden")
            }
            viewUploader = !viewUploader
        }
    })()

    // Sticker Preview
    function preview_update() {
        if (!preview || !formS || !formY || !formX || !elemCanvas || !elemPreview) return

        let max = Math.floor((CANVAS_STICKER_MAX_HEIGHT / ih) * 100)
        if (max > 100) max = 100
        formS.max = String(max)
        formS.removeAttribute("disabled")

        formX.min = String(0 - Math.round(iw * scale))
        formX.max = String(elemCanvas.offsetWidth)
        formX.removeAttribute("disabled")

        formY.min = String(0 - Math.round(ih * scale))
        formY.max = String(elemCanvas.offsetHeight)
        formY.removeAttribute("disabled")

        elemPreview.style.opacity = "1"
        elemPreview.style.height = `${Math.round(ih * scale)}px`
        elemPreview.style.width = `${Math.round(iw * scale)}px`
        elemPreview.style.bottom = `${oy}px`
        elemPreview.style.left = `${ox}px`
    }
    if (formX) formX.oninput = () => {
        const e = formX, v = e.valueAsNumber
        if (isNaN(v)) return
        if (v < parseInt(e.min)) e.value = e.min
        if (v > parseInt(e.max)) e.value = e.max
        ox = e.valueAsNumber
        preview_update()
    }
    if (formY) formY.oninput = () => {
        const e = formY, v = e.valueAsNumber
        if (isNaN(v)) return
        if (v < parseInt(e.min)) e.value = e.min
        if (v > parseInt(e.max)) e.value = e.max
        oy = e.valueAsNumber
        preview_update()
    }
    if (formS) formS.oninput = () => {
        const e = formS, v = e.valueAsNumber
        if (isNaN(v)) return
        if (v < parseInt(e.min)) e.value = e.min
        if (v > parseInt(e.max)) e.value = e.max
        scale = e.valueAsNumber / 100
        preview_update()
    }

    if (formFile) formFile.onchange = async () => {
        if (!elemPreview || !formS || !formX || !formY) return
        const image = formFile.files?.item(0)
        if (!image) {
            elemPreview.style.opacity = "0"
            elemPreview.src = blank_pixel
            return
        }
        elemPreview.src = await file_url(image)
        elemPreview.onload = () => {
            iw = elemPreview.naturalWidth
            ih = elemPreview.naturalHeight
            if (ih > CANVAS_STICKER_MAX_HEIGHT_START) {
                scale = CANVAS_STICKER_MAX_HEIGHT_START / ih
            }
            ox = Math.round((CANVAS_WIDTH / 2) - ((iw * scale) / 2))
            oy = Math.round((CANVAS_HEIGHT / 2) - ((ih * scale / 2)))
            formS.valueAsNumber = Math.round(scale * 100)
            formX.valueAsNumber = ox
            formY.valueAsNumber = oy
            preview = true
            preview_update()
        }
    }

    /** @param {MouseEvent} ev */
    function preview_move(ev) {
        if (!preview || !formX || !formY || !elemPreview) return
        formX.valueAsNumber = ev.offsetX - Math.round((iw * scale) / 2)
        formY.valueAsNumber = CANVAS_HEIGHT - ev.offsetY - Math.round((ih * scale) / 2)
        if (formX.oninput) formX.oninput(ev)
        if (formY.oninput) formY.oninput(ev)
    }
    if (elemCanvas) {
        elemCanvas.onclick = preview_move
        document.onmouseup = () => { click = false }
        elemCanvas.onmousedown = () => { click = true }
        elemCanvas.onmousemove = e => { click && preview_move(e) }
    }

    // Sticker Forms
    function canvas_clear() {
        if (
            !formX || !formY || !formS || !formComment || !formUserName ||
            !formUserURL || !formFile || !elemPreview || !elemPreview || !formError
        ) return
        for (const elem of [formX, formY, formS]) {
            elem.setAttribute("disabled", "true")
            elem.value = "0"
        }
        formS.value = "100"
        formComment.value = ""
        formUserName.value = ""
        formUserURL.value = ""
        formFile.value = ""
        elemPreview.style.opacity = "0"
        elemPreview.src = blank_pixel
        formError.textContent = "..."
        preview = false
    }
    if (formSubmit) formSubmit.onclick = async () => {
        if (
            busy || !formX || !formY || !formS || !elemCanvas || !formError ||
            !formFile || !formUserURL || !formUserName || !formComment
        ) return
        busy = true
        try {
            // Sanity Checks
            //  Comment, X, Y, and Scale already accounted for
            const image = formFile.files?.item(0)
            if (!image) {
                throw "Missing Sticker"
            }
            if (formUserURL.value && !safe_url(formUserURL.value)) {
                throw "Invalid URL"
            }

            // Generate Form Body
            const form = new FormData()
            form.append("sticker", image, "sticker")
            form.append("data", JSON.stringify({
                offset_x: formX.valueAsNumber,
                offset_y: formY.valueAsNumber,
                image_scale: formS.valueAsNumber,
                user_url: formUserURL.value,
                user_name: formUserName.value,
                message: formComment.value,
            }))

            // Send Image
            formError.textContent = "Uploading, please wait..."
            const resp = await fetch("/stickers", { method: "POST", body: form })
            if (resp.status !== 201) {
                throw `${resp.status}: ${await resp.text() || resp.statusText}`
            }
            formError.textContent = "Done! Refreshing..."
            window.location.reload()

        } catch (err) {
            console.error(err)
            formError.textContent = String(err)
        }
        busy = false
    }

    // Sticker Hovering
    document.querySelectorAll(".section-post").forEach(elem => {
        const x = parseInt(elem.getAttribute("data-offsetx") || "0")
        const y = parseInt(elem.getAttribute("data-offsety") || "0")
        const s = parseFloat(elem.getAttribute("data-scale") || "0")
        const h = parseInt(elem.getAttribute("data-height") || "0")
        const w = parseInt(elem.getAttribute("data-width") || "0")
        elem.addEventListener("mouseover", () => {
            if (elemPreview) {
                elemPreview.style.opacity = "1"
                elemPreview.style.height = `${Math.round(h * s)}px`
                elemPreview.style.width = `${Math.round(w * s)}px`
                elemPreview.style.bottom = `${y}px`
                elemPreview.style.left = `${x}px`
            }
        })
        elem.addEventListener("mouseout", () => {
            if (elemPreview) {
                elemPreview.style.opacity = "0"
            }
        })
    })

})()