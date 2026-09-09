export function base64_url_encode(input) {
  let output = btoa(input)
  output = output.replaceAll('+', '.')
  output = output.replaceAll('/', '_')
  output = output.replaceAll('=', '-')
  return output
}

export function base64_url_decode(input) {
  let output = input.replaceAll('-', '=')
  output = output.replaceAll('_', '/')
  output = output.replaceAll('.', '+')
  return atob(output)
}