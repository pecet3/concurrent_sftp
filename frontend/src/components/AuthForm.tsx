import { useState } from "react";

export const AuthForm = () => {
  const [formData, setFormData] = useState({
    name: "",
    email: "",
  });
  const [isSignUp, setIsSignUp] = useState(false);
  const [mailboxName, setMailboxName] = useState("");

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData((prevState) => ({
      ...prevState,
      [name]: value,
    }));
  };

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();

    if (isSignUp) {
      const response = await fetch("/api/v1/auth/register", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(formData),
      });

      if (response.ok) {
        setMailboxName(getMailboxName(formData.email));
      } else {
      }
    } else {
      const response = await fetch("/api/v1/auth/send-magic-link", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ email: formData.email }),
      });

      if (response.ok) {
        setMailboxName(getMailboxName(formData.email));
      } else {
      }
    }
  };
  const getMailProvider = (email: string) => {
    const domain = email.split("@")[1];
    switch (domain) {
      case "gmail.com":
        return "https://mail.google.com";
      case "yahoo.com":
        return "https://mail.yahoo.com";
      case "outlook.com":
      case "hotmail.com":
        return "https://outlook.com";
      default:
        return "https://mail." + domain;
    }
  };

  const getMailboxName = (email: string) => {
    const domain = email.split("@")[1];
    switch (domain) {
      case "gmail.com":
        return "Gmail";
      case "yahoo.com":
        return "Yahoo Mail";
      case "outlook.com":
        return "Outlook";
      case "hotmail.com":
        return "Hotmail";
      default:
        return domain;
    }
  };
  return (
    <form
      onSubmit={handleSubmit}
      className="bg-white p-4 flex  flex-col items-center text-black rounded shadow-md w-96"
    >
      <div className="mb-4">
        {isSignUp ? (
          <>
            <label
              htmlFor="name"
              className="block mb-2 text-sm font-medium text-gray-600"
            >
              Name
            </label>
            <input
              type="text"
              id="name"
              name="name"
              value={formData.name}
              onChange={handleChange}
              className="w-full px-3 py-2 border text-black
                         border-gray-300 rounded-md focus:outline-none focus:ring focus:border-blue-300"
              required
            />
            <label
              htmlFor="email"
              className="block mb-2 text-sm font-medium text-gray-600"
            >
              Email
            </label>
            <input
              type="email"
              id="email"
              name="email"
              value={formData.email}
              onChange={handleChange}
              className="w-full px-3 py-2 border text-black
                         border-gray-300 rounded-md focus:outline-none focus:ring focus:border-blue-300"
              required
            />
          </>
        ) : (
          <>
            <label
              htmlFor="email"
              className="block mb-2 text-sm font-medium text-gray-600"
            >
              Email
            </label>
            <input
              type="email"
              id="email"
              name="email"
              value={formData.email}
              onChange={handleChange}
              className="w-full px-3 py-2 border text-black
                         border-gray-300 rounded-md focus:outline-none focus:ring focus:border-blue-300"
              required
            />
          </>
        )}
      </div>
      {isSignUp ? (
        <>
          <button
            type="submit"
            className="min-w-64 bg-blue-500 text-white py-2 px-4 rounded-md hover:bg-blue-600 focus:outline-none focus:ring focus:border-blue-300"
          >
            Sign Up
          </button>
        </>
      ) : (
        <>
          <button
            type="submit"
            className="min-w-64 bg-blue-500 text-white py-2 px-4 rounded-md hover:bg-blue-600 focus:outline-none focus:ring focus:border-blue-300"
          >
            Sign In
          </button>
        </>
      )}
      <div className="flex justify-end w-full hover:cursor-pointer">
        {" "}
        {!isSignUp ? (
          <div onClick={() => setIsSignUp((prev) => !prev)}>Sign Up</div>
        ) : (
          <div onClick={() => setIsSignUp((prev) => !prev)}>Sign In</div>
        )}
      </div>
    </form>
  );
};
