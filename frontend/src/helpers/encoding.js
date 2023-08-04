export function base64_url_encode(input) {
  let output = btoa(input)
  output = output.replace('+', '.')
  output = output.replace('/', '_')
  output = output.replace('=', '-')
  return output
}