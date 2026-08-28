# Operational rules

## Job payloads

- Layer names are case-sensitive. Use the `name` returned by `template layers` as `layerName`.
- Codec identifiers use their full prefix, such as `video_h264_vbr_15mbps`, `image_png`, or `video_prores_422`.
- `preview: true` and `settings` are mutually exclusive. Preview mode ignores normal output settings.
- `${secrets.NAME}` is the secret reference form.
- `missingFonts` is a visual-correctness warning. A render may finish with fallback fonts and still be wrong.

## Template uploads

`template upload` performs the complete sequence. It creates the template record, uploads the file to the presigned URL without a Nexrender bearer header, and polls the v3 template endpoint until the template is usable.

Do not reproduce the presigned upload with an authenticated API client. The storage host must not receive the Nexrender token.

## Diagnosis

`job diagnose` is read-only. It combines job state, render logs, the recorded error, missing-font warnings, and available template metadata. Use its evidence before proposing a payload change or resubmission.

Terminal job states include `finished`, `error`, and `manually_cancelled`. Only retry after identifying the cause and receiving authorization for another paid render.
