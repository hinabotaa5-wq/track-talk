import type { Product } from "@/lib/types";
import { productKindLabels } from "@/lib/product";

export function ProductImage({ product, className = "aspect-[4/3]" }: { product: Product; className?: string }) {
  if (product.image_url) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={product.image_url} alt="" className={`${className} w-full bg-white object-contain`} />;
  }
  return (
    <div className={`${className} flex w-full items-center justify-center bg-gradient-to-br from-white/10 to-sky-400/10 px-3 text-center text-xs font-semibold text-slate-300`}>
      {product.brand}
      <br />
      {productKindLabels[product.kind]}
    </div>
  );
}
