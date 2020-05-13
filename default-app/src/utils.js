export function ZeroPad(num) {
    if (num < 10) {
        return '0' + num
    }
    return num + ''
}

export function FormatDate(date) {
    if (typeof date == 'string') {
        date = new Date(date)
    }

    return date.getFullYear() + '-' + ZeroPad(date.getMonth() + 1) + '-' + ZeroPad(date.getDate()) +
        ' ' + ZeroPad(date.getHours()) + ':' + ZeroPad(date.getMinutes()) + ':' + ZeroPad(date.getSeconds())
}

const escaped = {
    '"': '&quot;',
    "'": '&#39;',
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
}

// Escape HTML characters
// Source: https://github.com/sveltejs/svelte/blob/a0c934d0b016acc6fc95e6634fd8130ffbdb6a35/src/compiler/compile/utils/stringify.ts#L22
export function EscapeHTML(html) {
    return String(html).replace(/["'&<>]/g, match => escaped[match])
}