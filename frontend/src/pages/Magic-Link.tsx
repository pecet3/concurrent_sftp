import { useEffect } from "react";
import { useParams } from "react-router-dom";

export function MagicLinks() {
  const { code } = useParams();

  useEffect(() => {
    const activate = async () => {
      const id = code;
      console.log(id);
      if (id) {
        const response = await fetch(`/api/auth/magic-links/${id}`, {
          method: "GET",
          credentials: "include",
        });
        if (response.ok) {
          console.log("OK");
        }
      }
    };
    activate();
  }, []);

  return (
    <div className="flex justify-center items-center min-h-screen">
      <p>Logging</p>
    </div>
  );
}
