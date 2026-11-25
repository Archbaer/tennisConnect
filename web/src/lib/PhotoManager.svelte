<script lang="ts">
  import { onMount } from 'svelte';
  import { ApiError, apiJSON, type Photo } from '#lib/api';
  import { online } from '#lib/offline';

  export let photos: Photo[] = [];
  export let onChange: (() => void) | undefined = undefined;

  let error = '';
  let busy = false;
  let uploading = false;
  let processing = false;
  let stage = '';
  let progress = 0;
  let previewUrl = '';
  let pendingFile: File | null = null;
  let uploadFailed = false;
  let cancelled = false;
  let request: XMLHttpRequest | null = null;
  let fileInput: HTMLInputElement;
  let urls: Record<string, string> = {};
  let announcement = '';
  let lastStatuses = new Map<string, Photo['status']>();

  const slots = [1, 2, 3];
  const maxBytes = 5 * 1024 * 1024;
  const maxPixels = 20_000_000;

  onMount(() => {
    refresh();
    const timer = window.setInterval(refresh, 15_000);
    return () => window.clearInterval(timer);
  });

  function photoAt(order: number): Photo | undefined {
    return photos.find((photo) => photo.display_order === order);
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

  async function validate(file: File): Promise<string> {
    if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
      return 'We can’t read that file. Use a JPEG, PNG or WebP photo. On iPhone: Settings › Camera › Formats › Most Compatible.';
    }
    if (file.size > maxBytes) {
      return `That photo is ${(file.size / (1024 * 1024)).toFixed(1)} MB. The limit is 5 MB. Choose a smaller one.`;
    }
    try {
      const bitmap = await createImageBitmap(file);
      const pixels = bitmap.width * bitmap.height;
      bitmap.close();
      if (pixels > maxPixels) {
        return `That photo is ${(pixels / 1_000_000).toFixed(0)} megapixels. The limit is 20. Choose a smaller version.`;
      }
    } catch {
      return 'We couldn’t read that image. Choose a different file.';
    }
    return '';
  }

  async function selectFile(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (file) await uploadFile(file);
    if (fileInput) fileInput.value = '';
  }

  async function uploadFile(file: File) {
    error = '';
    cancelled = false;
    const validation = await validate(file);
    if (validation) {
      error = validation;
      return;
    }

    pendingFile = file;
    if (previewUrl) URL.revokeObjectURL(previewUrl);
    previewUrl = URL.createObjectURL(file);
    uploadFailed = false;
    busy = true;
    uploading = true;
    processing = false;
    progress = 0;
    stage = 'Uploading…';

    try {
      const slot = await apiJSON<{ photo_id: string; upload_url: string }>(
        '/api/v1/photos/upload-url',
        { method: 'POST' }
      );
      await putWithProgress(slot.upload_url, file);
      uploading = false;
      processing = true;
      stage = 'Checking…';
      await apiJSON(`/api/v1/photos/${slot.photo_id}/process`, { method: 'POST' });
      await refresh(false);
      resetUpload();
    } catch (problem) {
      if (cancelled) {
        cancelled = false;
        return;
      }
      if (problem instanceof ApiError && problem.status === 503) {
        if (uploading) {
          error = 'Photo uploads aren’t available right now. Try again in a minute.';
          uploadFailed = true;
        } else {
          error = 'We couldn’t check that photo just now. Try again.';
          resetUpload();
          await refresh(false);
        }
      } else if (problem instanceof ApiError && problem.status === 409) {
        error = 'That upload took too long, so we cleared it. Add the photo again.';
        resetUpload();
      } else {
        error = problem instanceof Error ? problem.message : 'We could not process that photo.';
        uploadFailed = true;
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
    busy = false;
    uploading = false;
    processing = false;
    stage = '';
  }

  function retryUpload() {
    if (pendingFile) void uploadFile(pendingFile);
  }

  function resetUpload() {
    if (previewUrl) URL.revokeObjectURL(previewUrl);
    previewUrl = '';
    pendingFile = null;
    progress = 0;
    uploadFailed = false;
    processing = false;
  }

  async function checkAgain(photo: Photo) {
    error = '';
    busy = true;
    stage = 'Checking again…';
    try {
      await apiJSON(`/api/v1/photos/${photo.photo_id}/process`, { method: 'POST' });
      await refresh();
    } catch (problem) {
      if (problem instanceof ApiError && problem.status === 409) {
        error = 'That upload took too long, so we cleared it. Add the photo again.';
      } else if (problem instanceof ApiError && problem.status === 503) {
        error = 'We couldn’t check that photo just now. Try again.';
      } else {
        error = problem instanceof Error ? problem.message : 'We could not check that photo.';
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
    <div class="upload-preview">
      <img src={previewUrl} alt="Upload preview" />
      <div class="upload-copy">
        <span class="chip" class:alert={uploadFailed}>{uploadFailed ? 'Upload failed' : stage || (processing ? 'Checking…' : 'Ready')}</span>
        {#if uploading}
          <div class="progress" role="progressbar" aria-label="Photo upload" aria-valuenow={progress} aria-valuemin="0" aria-valuemax="100">
            <div class="progress-bar" style={`width: ${progress}%`}></div>
          </div>
          <button class="btn btn-outline-secondary btn-sm" type="button" onclick={cancelUpload}>Cancel</button>
        {:else if uploadFailed}
          <button class="btn btn-outline-secondary btn-sm" type="button" onclick={retryUpload} disabled={!$online}>Try again</button>
        {/if}
      </div>
    </div>
  {/if}

  <div class="slots" role="list">
    {#each slots as order}
      {@const photo = photoAt(order)}
      <div class="slot" role="listitem">
        {#if photo && photo.status !== 'rejected' && urls[photo.photo_id]}
          <img class="slot-photo" src={urls[photo.photo_id]} alt={`Photo ${order}`} />
        {:else if photo && photo.status === 'rejected'}
          <div class="slot-photo slot-status"><i class="bi bi-exclamation-triangle" aria-hidden="true"></i></div>
        {:else}
          <div class="slot-photo slot-empty"><i class="bi bi-plus-lg" aria-hidden="true"></i></div>
        {/if}
        <div class="d-flex justify-content-between align-items-center mt-2">
          <span class="score">{order === 1 ? 'Main photo' : `Photo ${order}`}</span>
          {#if photo}
            <span class:alert={photo.status === 'rejected'} class="chip">{photo.status === 'pending' ? 'Not confirmed' : photo.status === 'rejected' ? 'Not accepted' : 'Approved'}</span>
          {/if}
        </div>
        {#if photo?.status === 'pending'}
          <button class="btn btn-outline-secondary btn-sm mt-2 w-100" onclick={() => checkAgain(photo)} disabled={busy || !$online}>
            Check again
          </button>
        {/if}
      </div>
    {/each}
  </div>

  {#if error}
    <p class="alert alert-danger mb-0" role="alert">{error}</p>
  {/if}

  <label>
    <span class="btn btn-primary {busy || photos.length >= 3 || !$online ? 'disabled' : ''}">
      <i class="bi bi-camera" aria-hidden="true"></i> {busy ? stage || 'Working…' : photos.length >= 3 ? '3 photos added' : 'Add a photo'}
    </span>
    <input
      class="visually-hidden"
      bind:this={fileInput}
      type="file"
      accept="image/jpeg,image/png,image/webp"
      onchange={selectFile}
      disabled={busy || photos.length >= 3 || !$online}
    />
    <small>The server validates, strips location metadata and moderates the image.</small>
  </label>
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
    background: #e5ebe6;
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
    background: #f7e3dd;
    color: var(--clay-deep);
    border-style: solid;
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

  .upload-copy {
    display: grid;
    justify-items: start;
    gap: 0.5rem;
  }

  .progress {
    width: 100%;
    height: 0.5rem;
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
