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

    return date.getFullYear() + '-' + ZeroPad(date.getMonth() + 1) + '-' + ZeroPad(date.getDay()) +
        ' ' + ZeroPad(date.getHours()) + ':' + ZeroPad(date.getMinutes()) + ':' + ZeroPad(date.getSeconds())
}
