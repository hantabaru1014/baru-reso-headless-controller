import type { Area } from "react-easy-crop";

const ICON_SIZE = 256;

/**
 * Process an image for use as a user icon.
 * Crops the image based on the provided crop area and resizes to 256x256.
 * Returns a PNG blob.
 */
export async function processIconImage(
  imageSrc: string,
  cropArea: Area,
): Promise<Blob> {
  const img = await loadImageFromUrl(imageSrc);

  // Create canvas for processing
  const canvas = document.createElement("canvas");
  canvas.width = ICON_SIZE;
  canvas.height = ICON_SIZE;
  const ctx = canvas.getContext("2d");
  if (!ctx) {
    throw new Error("Failed to get canvas context");
  }

  // Draw the cropped and resized image
  ctx.drawImage(
    img,
    cropArea.x,
    cropArea.y,
    cropArea.width,
    cropArea.height,
    0,
    0,
    ICON_SIZE,
    ICON_SIZE,
  );

  // Convert to PNG blob
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => {
        if (blob) {
          resolve(blob);
        } else {
          reject(new Error("Failed to create blob from canvas"));
        }
      },
      "image/png",
      1.0,
    );
  });
}

/**
 * Load an image from a URL and return an HTMLImageElement.
 */
function loadImageFromUrl(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();

    img.onload = () => {
      resolve(img);
    };

    img.onerror = () => {
      reject(new Error("Failed to load image"));
    };

    img.src = url;
  });
}

/**
 * Create an object URL from a File.
 * The caller is responsible for revoking the URL when done.
 */
export function createImageUrl(file: File): string {
  return URL.createObjectURL(file);
}
