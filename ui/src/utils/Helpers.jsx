import moment from 'moment'
import { apiClient } from './ApiClient'

const isPlusAccount = userProfile => {
  return userProfile?.expiration && moment(userProfile?.expiration).isAfter()
}

const resolvePhotoURL = url => {
  if (!url) return ''
  if (url.startsWith('http') || url.startsWith('https')) {
    return url
  }
  if (url.startsWith('assets')) {
    return apiClient.getAssetURL(url)
  }
  return url
}

// Copy text to the clipboard, reporting whether it actually landed there.
// Never throws: callers decide what to show based on the boolean.
const copyText = async value => {
  if (!value) return false

  if (navigator?.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value)
      return true
    } catch {
      return false
    }
  }

  // No async clipboard API (insecure context, old webview). Fall back to a
  // hidden textarea and the legacy command.
  let textarea
  try {
    textarea = document.createElement('textarea')
    textarea.value = value
    textarea.setAttribute('readonly', '')
    textarea.style.position = 'fixed'
    textarea.style.top = '-1000px'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    textarea.setSelectionRange(0, value.length)
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    if (textarea?.parentNode) {
      textarea.parentNode.removeChild(textarea)
    }
  }
}

export { copyText, isPlusAccount, resolvePhotoURL }
