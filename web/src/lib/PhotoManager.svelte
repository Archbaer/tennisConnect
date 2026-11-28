<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError, apiJSON, type Photo } from '#lib/api';
  import { online } from '#lib/offline';
  import {
    addControlText,
    photoStatusCopy,
    slotLimitCopy,
    uploadFailureCopy,
    type PhotoAction,
    type UploadFailure,
    type UploadFailureDetail
  } from '#lib/photoState';
  import { showToast } from '#lib/toasts';

  export let photos: Photo[] = [];
  export let onChange: (() => void) | undefined = undefined;

  let busy = false;
  let uploading = false;
  let processing = false;
  let stage = '';
  let progress = 0;
  let previewUrl = '';
  let pendingFile: File | null = null;
  let failure: UploadFailure | '' = '';
  let failureDetail: UploadFailureDetail = {};
  let cancelled = false;
  let request: XMLHttpRequest | null = null;
  let fileInput: HTMLInputElement;
  let urls: Record<string, string> = {};
  let announcement = '';
  let lastStatuses = new Map<string, Photo['status']>();
  let slotLimit = false;

  const slots = [1, 2, 3];
  const maxBytes = 5 * 1024 * 1024;
  const maxPixels = 20_000_000;

  $: atSlotLimit = photos.length >= 3 || slotLimit;

  onMount(() => {
    refresh();
    const timer = window.setInterval(refresh, 15_000);
    return () => window.clearInterval(timer);
  });

  function photoAt(order: number): Photo | undefined {
    return photos.find((photo) => photo.display_order === order);
  }

  function actionLabel(action: PhotoAction): string {
    switch (action) {
      case 'check_again':
        return 'Check again';
      case 'upload_another':
        return 'Upload another photo';
      case 'retry':
        return 'Retry';
      case 'choose_another':
        return 'Choose another';
      default:
        return 'Try again';
    }
  }

  function openPicker() {
    fileInput?.click();
  }

  function runAction(action: PhotoAction) {
    if (action === 'check_again') return;
    if (action === 'try_again' || action === 'retry') {
      if (pendingFile) void uploadFile(pendingFile);
      else openPicker();
      return;
    }
    openPicker();
  }

  async function refresh(announce = true) {
    try {
      const result = await apiJSON<{ photos: Photo[] }>('/api/v1/photos');
      const changes: string[] = [];
      if (announce) {
        for (const photo of result.photos) {
          const previous = lastStatuses.get(photo.photo_id);
          if (previous && previous !== photo.status) {
            changes.push(
              `Photo ${photo.display_order} was ${photo.status === 'approved' ? 'approved' : photo.status === 'rejected' ? 'not accepted' : 'not confirmed'}.`
            );
          }
        }
      }
      lastStatuses = new Map(result.photos.map((photo) => [photo.photo_id, photo.status]));
      photos = result.photos;
      const next: Record<string, string> = {};
      for (const photo of photos) {
        // Rejected objects are deleted by the server; never request their image.
        if (photo.status === 'rejected') continue;
        try {
          const url = await apiJSON<{ url: string }>(`/api/v1/photos/${photo.photo_id}/url`);
          next[photo.photo_id] = url.url;
        } catch {
          // A missing thumbnail is not a page failure.
        }
      }
      urls = next;
      if (changes.length) announcement = changes.join(' ');
      onChange?.();
    } catch {
      // A missing storage configuration is reported when an upload is attempted.
    }
  }

  type ValidationResult = { failure?: UploadFailure; detail?: UploadFailureDetail };

  async function validate(file: File): Promise<ValidationResult> {
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      return { failure: 'wrong_type' };
    }
    if (file.size > maxBytes) {
      return { failure: 'too_large', detail: { megabytes: file.size / (1024 * 1024) } };
    }
    try {
      const bitmap = await createImageBitmap(file);
      const pixels = bitmap.width * bitmap.height;
      bitmap.close();
      if (pixels > maxPixels) {
        return { failure: 'too_large', detail: { megapixels: pixels / 1_000_000 } };
      }
    } catch {
      return { failure: 'unreadable' };
    }
    return {};
  }

  async function selectFile(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (file) await uploadFile(file);
    if (fileInput) fileInput.value = '';
  }

  async function uploadFile(file: File) {
    cancelled = false;
    const validation = await validate(file);
    if (validation.failure) {
      resetUpload();
      failure = validation.failure;
      failureDetail = validation.detail ?? {};
      return;
    }

    pendingFile = file;
    if (previewUrl) URL.revokeObjectURL(previewUrl);
    previewUrl = URL.createObjectURL(file);
    failure = '';
    failureDetail = {};
    slotLimit = false;
    busy = true;
    uploading = true;
    processing = false;
    progress = 0;
    stage = 'Uploading…';

    try {
      let slot: { photo_id: string; upload_url: string };
      try {
        slot = await apiJSON<{ photo_id: string; upload_url: string }>(
          '/api/v1/photos/upload-url',
          { method: 'POST' }
        );
      } catch (problem) {
        if (problem instanceof ApiError && problem.status === 409) {
          slotLimit = true;
          resetUpload();
          await refresh(false);
          return;
        }
        if (problem instanceof ApiError && problem.status === 503) {
          failure = 'storage_unavailable';
          return;
        }
        throw problem;
      }
      await putWithProgress(slot.upload_url, file);
      uploading = false;
      processing = true;
      stage = 'Checking…';
      try {
        await apiJSON(`/api/v1/photos/${slot.photo_id}/process`, { method: 'POST' });
      } catch (problem) {
        if (problem instanceof ApiError && problem.status === 409) {
          failure = 'expired';
          resetUpload();
          await refresh(false);
          return;
        }
        if (problem instanceof ApiError && problem.status === 503) {
          // The preview and the file stay so the owner can retry by hand.
          failure = 'check_failed';
          return;
        }
        if (problem instanceof ApiError && problem.status === 400) {
          failure = 'unreadable';
          resetUpload();
          await refresh(false);
          return;
        }
        throw problem;
      }
      await refresh(false);
      resetUpload();
      showToast("Photo uploaded. It's being checked.");
    } catch (problem) {
      if (cancelled) {
        cancelled = false;
        return;
      }
      failure = 'upload_failed';
      if (problem instanceof ApiError && problem.status === 503) {
        failure = uploading ? 'upload_failed' : 'check_failed';
      }
    } finally {
      busy = false;
      uploading = false;
      processing = false;
      stage = '';
    }
  }

  function putWithProgress(url: string, file: File): Promise<void> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      request = xhr;
      xhr.open('PUT', url);
      xhr.setRequestHeader('Content-Type', file.type);
      xhr.upload.onprogress = (event) => {
        if (!event.lengthComputable) return;
        progress = Math.round((event.loaded / event.total) * 100);
        stage = `Uploading ${progress}%`;
      };
      xhr.onload = () => {
        request = null;
        if (xhr.status >= 200 && xhr.status < 300) resolve();
        else reject(new Error('The upload didn’t finish. Check your connection and try again.'));
      };
      xhr.onerror = () => {
        request = null;
        reject(new Error('The upload didn’t finish. Check your connection and try again.'));
      };
      xhr.onabort = () => {
        request = null;
        reject(new Error('Upload cancelled.'));
      };
      xhr.send(file);
    });
  }

  function cancelUpload() {
    cancelled = true;
    request?.abort();
    resetUpload();
    failure = '';
    busy = false;
    uploading = false;
    processing = false;
    stage = '';
  }

  function resetUpload() {
    if (previewUrl) URL.revokeObjectURL(previewUrl);
    previewUrl = '';
    pendingFile = null;
    progress = 0;
    processing = false;
  }

  async function checkAgain(photo: Photo) {
    busy = true;
    stage = 'Checking again…';
    try {
      await apiJSON(`/api/v1/photos/${photo.photo_id}/process`, { method: 'POST' });
      failure = '';
      await refresh();
    } catch (problem) {
      if (problem instanceof ApiError && problem.status === 409) {
        failure = 'expired';
        resetUpload();
        await refresh(false);
      } else if (problem instanceof ApiError && problem.status === 503) {
        failure = 'check_failed';
      } else {
        failure = 'upload_failed';
      }
    } finally {
      busy = false;
      stage = '';
    }
  }
</script>

<section class="surface stack" aria-labelledby="photos-title">
  <div>
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Up to 3 photos</p>
    <h2 id="photos-title">Photo check</h2>
    <p class="muted mb-0">
      JPEG, PNG or WebP, up to 5 MB and 20 MP. One approved photo puts you in Discover.
    </p>
  </div>

  <p class="visually-hidden" role="status" aria-live="polite">{announcement}</p>

  {#if previewUrl}
    {@const copy = failure ? uploadFailureCopy(failure, failureDetail) : null}
    <div class="upload-preview">
      <img src={previewUrl} alt="Upload preview" onerror={(event) => ((event.currentTarget as HTMLImageElement).hidden = true)} />
      <div class="upload-copy">
        {#if copy}
          <span class="chip alert">{copy.chip}</span>
          <p class="mb-0" role="alert">{copy.message}</p>
          {#if failure !== 'storage_unavailable'}
            <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => runAction(copy.action)} disabled={!$online}>
              {actionLabel(copy.action)}
            </button>
          {/if}
        {:else}
          <span class="chip">{stage || (processing ? 'Checking…' : 'Ready')}</span>
          {#if uploading}
            <progress class="progress-track" value={progress} max="100" aria-label="Photo upload"></progress>
            <button class="btn btn-outline-secondary btn-sm" type="button" onclick={cancelUpload}>Cancel</button>
          {/if}
        {/if}
      </div>
    </div>
  {/if}

  {#if failure && !previewUrl}
    {@const copy = uploadFailureCopy(failure, failureDetail)}
    <div class="upload-status" role="alert">
      <span class="chip alert">{copy.chip}</span>
      <p class="mb-0">{copy.message}</p>
      <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => runAction(copy.action)} disabled={!$online}>
        {actionLabel(copy.action)}
      </button>
    </div>
  {/if}

  <div class="slots" role="list">
    {#each slots as order}
      {@const photo = photoAt(order)}
      {@const copy = photo ? photoStatusCopy(photo.status) : null}
      <div class="slot" role="listitem">
        {#if photo && photo.status !== 'rejected' && urls[photo.photo_id]}
          <img
            class="slot-photo"
            src={urls[photo.photo_id]}
            alt={`Photo ${order}`}
            onerror={(event) => ((event.currentTarget as HTMLImageElement).hidden = true)}
          />
        {:else if photo && photo.status === 'rejected'}
          <div class="slot-photo slot-status"><i class="bi bi-exclamation-triangle" aria-hidden="true"></i></div>
        {:else}
          <div class="slot-photo slot-empty"><i class="bi bi-plus-lg" aria-hidden="true"></i></div>
        {/if}
        <div class="d-flex justify-content-between align-items-center mt-2">
          <span class="score">{order === 1 ? 'Main photo' : `Photo ${order}`}</span>
          {#if copy}
            <span class:alert={photo?.status === 'rejected'} class="chip">{copy.chip}</span>
          {/if}
        </div>
        {#if photo && copy}
          <p class="slot-copy">{copy.message}</p>
          {#if copy.action === 'check_again'}
            <button class="btn btn-outline-secondary btn-sm mt-2 w-100" onclick={() => checkAgain(photo)} disabled={busy || !$online}>
              Check again
            </button>
          {:else if copy.action === 'upload_another'}
            <button class="btn btn-outline-secondary btn-sm mt-2 w-100" onclick={openPicker} disabled={busy || !$online}>
              Upload another photo
            </button>
          {/if}
        {/if}
      </div>
    {/each}
  </div>

  {#if atSlotLimit}
    <p class="slot-limit mb-0">{slotLimitCopy}</p>
  {:else if failure === 'storage_unavailable'}
    <div class="upload-unavailable">
      <p class="btn btn-primary disabled mb-0" aria-disabled="true">Add a photo</p>
      <p class="muted mb-0">{uploadFailureCopy('storage_unavailable').message}</p>
      <button class="btn btn-outline-secondary btn-sm" type="button" onclick={() => runAction('retry')} disabled={!$online}>
        Retry
      </button>
    </div>
  {:else}
    <label for="photo-upload-input">
      <span class="btn btn-primary {busy || !$online ? 'disabled' : ''}">
        <i class="bi bi-camera" aria-hidden="true"></i> {addControlText(photos.length, busy, stage)}
      </span>
      <small>The server validates, strips location metadata and moderates the image.</small>
    </label>
  {/if}
  <input
    id="photo-upload-input"
    class="visually-hidden"
    bind:this={fileInput}
    type="file"
    accept="image/jpeg,image/png,image/webp"
    onchange={selectFile}
    disabled={busy || !$online}
  />
</section>

<style>
  .slots {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 0.65rem;
  }

  .slot {
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    padding: 0.5rem;
    background: var(--paper);
  }

  .slot-photo {
    width: 100%;
    aspect-ratio: 1;
    object-fit: cover;
    border-radius: calc(var(--radius-control) - 4px);
    background: var(--media-bg);
  }

  .slot-empty,
  .slot-status {
    display: grid;
    place-items: center;
    border: 1px dashed var(--control-border);
    color: var(--muted);
    font-size: 1.4rem;
  }

  .slot-status {
    background: var(--danger-bg);
    color: var(--danger);
    border-style: solid;
  }

  .slot-copy {
    margin: 0.4rem 0 0;
    color: var(--muted);
    font-size: 0.78rem;
    line-height: 1.35;
  }

  .slot-limit {
    color: var(--muted);
    font-weight: 600;
  }

  .upload-preview {
    display: grid;
    grid-template-columns: 7rem 1fr;
    gap: 0.75rem;
    align-items: center;
    padding: 0.65rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: var(--paper);
  }

  .upload-preview img {
    width: 7rem;
    aspect-ratio: 1;
    object-fit: cover;
    border-radius: calc(var(--radius-control) - 4px);
  }

  .upload-copy,
  .upload-status,
  .upload-unavailable {
    display: grid;
    justify-items: start;
    gap: 0.5rem;
  }

  .upload-status,
  .upload-unavailable {
    padding: 0.65rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: var(--paper);
  }

  @media (max-width: 520px) {
    .slots {
      grid-template-columns: 1fr;
    }

    .slot-photo {
      aspect-ratio: 16 / 9;
    }
  }
</style>
