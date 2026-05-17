// BrandMark is the small icon paired with the "meatbag" wordmark in the top
// header. Renders the project's meatbag_icon.png served from the Vite public
// directory at the site root.
export function BrandMark({ size = 32 }: { size?: number }) {
  return (
    <img
      src="/meatbag_icon.png"
      alt="meatbag"
      width={size}
      height={size}
      className="brand-mark"
      style={{ display: "block", borderRadius: 5 }}
    />
  );
}
